package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/joshdk/go-junit"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

var serviceNameFlag string
var serviceVersionFlag string
var traceNameFlag string

var runtimeAttributes []attribute.KeyValue

func init() {
	flag.StringVar(&serviceNameFlag, "service-name", Junit2otlp, "OpenTelemetry Service Name to be used when sending traces and metrics for the jUnit report")
	flag.StringVar(&serviceVersionFlag, "service-version", "", "OpenTelemetry Service Version to be used when sending traces and metrics for the jUnit report")
	flag.StringVar(&traceNameFlag, "trace-name", Junit2otlp, "OpenTelemetry Trace Name to be used when sending traces and metrics for the jUnit report")

	// initialise runtime keys
	runtimeAttributes = []attribute.KeyValue{
		semconv.HostArchKey.String(runtime.GOARCH),
		semconv.OSNameKey.String(runtime.GOOS),
	}
}

func createIntCounter(meter metric.Meter, name string, description string) metric.Int64Counter {
	return metric.Must(meter).
		NewInt64Counter(
			name,
			metric.WithDescription(description),
		)
}

func createTracesAndSpans(ctx context.Context, srvName string, tracesProvides *sdktrace.TracerProvider, suites []junit.Suite) error {
	tracer := tracesProvides.Tracer(srvName)
	meter := global.Meter(srvName)

	scm := GetScm()
	if scm != nil {
		scmAttributes := scm.contributeAttributes()
		runtimeAttributes = append(runtimeAttributes, scmAttributes...)
	}

	durationCounter := createIntCounter(meter, TestsDuration, "Duration of the tests")
	errorCounter := createIntCounter(meter, ErrorTestsCount, "Total number of failed tests")
	failedCounter := createIntCounter(meter, FailedTestsCount, "Total number of failed tests")
	passedCounter := createIntCounter(meter, PassedTestsCount, "Total number of passed tests")
	skippedCounter := createIntCounter(meter, SkippedTestsCount, "Total number of skipped tests")
	testsCounter := createIntCounter(meter, TotalTestsCount, "Total number of executed tests")

	ctx, outerSpan := tracer.Start(ctx, traceNameFlag, trace.WithAttributes(runtimeAttributes...))
	defer outerSpan.End()

	for _, suite := range suites {
		totals := suite.Totals

		suiteAttributes := []attribute.KeyValue{
			semconv.CodeNamespaceKey.String(suite.Package),
			attribute.Key(TestsSuiteName).String(suite.Name),
			attribute.Key(TestsSystemErr).String(suite.SystemErr),
			attribute.Key(TestsSystemOut).String(suite.SystemOut),
			attribute.Key(TestsDuration).Int64(suite.Totals.Duration.Milliseconds()),
		}

		suiteAttributes = append(suiteAttributes, runtimeAttributes...)
		suiteAttributes = append(suiteAttributes, propsToLabels(suite.Properties)...)

		durationCounter.Add(ctx, totals.Duration.Milliseconds(), suiteAttributes...)
		errorCounter.Add(ctx, int64(totals.Error), suiteAttributes...)
		failedCounter.Add(ctx, int64(totals.Failed), suiteAttributes...)
		passedCounter.Add(ctx, int64(totals.Passed), suiteAttributes...)
		skippedCounter.Add(ctx, int64(totals.Skipped), suiteAttributes...)
		testsCounter.Add(ctx, int64(totals.Tests), suiteAttributes...)

		ctx, suiteSpan := tracer.Start(ctx, suite.Name,
			trace.WithAttributes(suiteAttributes...))
		for _, test := range suite.Tests {
			testAttributes := []attribute.KeyValue{
				semconv.CodeFunctionKey.String(test.Name),
				attribute.Key(TestDuration).Int64(test.Duration.Milliseconds()),
				attribute.Key(TestClassName).String(test.Classname),
				attribute.Key(TestMessage).String(test.Message),
				attribute.Key(TestStatus).String(string(test.Status)),
				attribute.Key(TestSystemErr).String(test.SystemErr),
				attribute.Key(TestSystemOut).String(test.SystemOut),
			}

			testAttributes = append(testAttributes, propsToLabels(test.Properties)...)
			testAttributes = append(testAttributes, suiteAttributes...)

			if test.Error != nil {
				testAttributes = append(testAttributes, attribute.Key(TestError).String(test.Error.Error()))
			}

			_, testSpan := tracer.Start(ctx, test.Name,
				trace.WithAttributes(testAttributes...))
			testSpan.End()
		}

		suiteSpan.End()
	}

	return nil
}

// getOtlpEnvVar checks if the env variable, removing the OTEL prefix, needs to override
func getOtlpEnvVar(key string, fallback string) string {
	envVar := os.Getenv(key)
	if envVar != "" {
		return envVar
	}

	return fallback
}

// getOtlpServiceName checks the service name
func getOtlpServiceName() string {
	return getOtlpEnvVar("OTEL_SERVICE_NAME", serviceNameFlag)
}

// getOtlpServiceVersion checks the service version
func getOtlpServiceVersion() string {
	return getOtlpEnvVar("OTEL_SERVICE_VERSION", serviceVersionFlag)
}

func initMetricsExporter(ctx context.Context) (*otlpmetric.Exporter, error) {
	exp, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create the collector exporter: %v", err)
	}

	return exp, nil
}

func initMetricsPusher(ctx context.Context, exporter *otlpmetric.Exporter, res *resource.Resource) (*controller.Controller, error) {
	pusher := controller.New(
		processor.NewFactory(
			simple.NewWithExactDistribution(),
			exporter,
		),
		controller.WithExporter(exporter),
		controller.WithCollectPeriod(2*time.Second),
		controller.WithResource(res),
	)
	global.SetMeterProvider(pusher)

	if err := pusher.Start(ctx); err != nil {
		return nil, fmt.Errorf("could not start metric controller: %v", err)
	}

	return pusher, nil
}

func initTracerProvider(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(traceExporter)),
	)
	return tracerProvider, nil
}

func propsToLabels(props map[string]string) []attribute.KeyValue {
	attributes := []attribute.KeyValue{}
	for k, v := range props {
		attributes = append(attributes, attribute.Key(k).String(v))
	}

	return attributes
}

type InputReader interface {
	Read() ([]byte, error)
}

type PipeReader struct{}

func (pr *PipeReader) Read() ([]byte, error) {
	stat, _ := os.Stdin.Stat()

	if (stat.Mode() & os.ModeCharDevice) == 0 {
		var buf []byte
		scanner := bufio.NewScanner(os.Stdin)

		for scanner.Scan() {
			buf = append(buf, scanner.Bytes()...)
		}

		if err := scanner.Err(); err != nil {
			return nil, err
		}

		return buf, nil
	}

	return nil, fmt.Errorf("there is no data in the pipe")
}

func Main(ctx context.Context, reader InputReader) error {
	otlpSrvName := getOtlpServiceName()
	otlpSrvVersion := getOtlpServiceVersion()

	// set the service name that will show up in tracing UIs
	resAttrs := resource.WithAttributes(
		semconv.ServiceNameKey.String(otlpSrvName),
		semconv.ServiceVersionKey.String(otlpSrvVersion),
	)
	res, err := resource.New(ctx, resAttrs)
	if err != nil {
		return fmt.Errorf("failed to create OpenTelemetry service name resource: %s", err)
	}

	tracesProvides, err := initTracerProvider(ctx, res)
	if err != nil {
		return err
	}
	defer tracesProvides.Shutdown(ctx)

	metricsExporter, err := initMetricsExporter(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialise metrics exporter: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if err := metricsExporter.Shutdown(ctx); err != nil {
			otel.Handle(err)
		}
	}()

	pusher, err := initMetricsPusher(ctx, metricsExporter, res)
	if err != nil {
		return fmt.Errorf("failed to initialise pusher: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		// pushes any last exports to the receiver
		if err := pusher.Stop(ctx); err != nil {
			otel.Handle(err)
		}
	}()

	xmlBuffer, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read from pipe: %v", err)
	}

	suites, err := junit.Ingest(xmlBuffer)
	if err != nil {
		return fmt.Errorf("failed to ingest JUnit xml: %v", err)
	}

	return createTracesAndSpans(ctx, otlpSrvName, tracesProvides, suites)
}

func main() {
	flag.Parse()

	if err := Main(context.Background(), &PipeReader{}); err != nil {
		log.Fatal(err)
	}
}
