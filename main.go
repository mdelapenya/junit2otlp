package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/joshdk/go-junit"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const defaultMaxBatchSize = 10

var batchSizeFlag int
var repositoryPathFlag string
var serviceNameFlag string
var serviceVersionFlag string
var traceNameFlag string
var propertiesAllowedString string

const propertiesAllowAll = "all"

var runtimeAttributes []attribute.KeyValue
var propsAllowed []string

func init() {
	flag.IntVar(&batchSizeFlag, "batch-size", defaultMaxBatchSize, "Maximum export batch size allowed when creating a BatchSpanProcessor")
	flag.StringVar(&repositoryPathFlag, "repository-path", getDefaultwd(), "Path to the SCM repository to be read")
	flag.StringVar(&serviceNameFlag, "service-name", "", "OpenTelemetry Service Name to be used when sending traces and metrics for the jUnit report")
	flag.StringVar(&serviceVersionFlag, "service-version", "", "OpenTelemetry Service Version to be used when sending traces and metrics for the jUnit report")
	flag.StringVar(&traceNameFlag, "trace-name", Junit2otlp, "OpenTelemetry Trace Name to be used when sending traces and metrics for the jUnit report")
	flag.StringVar(&propertiesAllowedString, "properties-allowed", propertiesAllowAll, "Comma separated list of properties to be allowed in the jUnit report")

	// initialize runtime keys
	runtimeAttributes = []attribute.KeyValue{
		semconv.HostArchKey.String(runtime.GOARCH),
		semconv.OSNameKey.String(runtime.GOOS),
	}

	propsAllowed = []string{}
	if propertiesAllowedString != "" {
		allowed := strings.Split(propertiesAllowedString, ",")
		for _, prop := range allowed {
			propsAllowed = append(propsAllowed, strings.TrimSpace(prop))
		}
	}
}

func createIntCounter(meter metric.Meter, name string, description string) metric.Int64Counter {
	counter, _ := meter.Int64Counter(name, metric.WithDescription(description))
	// Accumulators always return nil errors
	// see https://github.com/open-telemetry/opentelemetry-go/blob/e8fbfd3ec52d8153eea3f13465b7de15cd8f6320/sdk/metric/sdk.go#L256-L264
	return counter
}

func createFloat64Histogram(meter metric.Meter, name string, description string, unit string) metric.Float64Histogram {
	histogram, _ := meter.Float64Histogram(name, metric.WithDescription(description), metric.WithUnit(unit))
	return histogram
}

func createTracesAndSpans(ctx context.Context, srvName string, tracesProvides *sdktrace.TracerProvider, suites []junit.Suite) error {
	tracer := tracesProvides.Tracer(srvName)
	meter := otel.Meter(srvName)

	scm := GetScm(repositoryPathFlag)
	if scm != nil {
		scmAttributes := scm.contributeAttributes()
		runtimeAttributes = append(runtimeAttributes, scmAttributes...)
	}

	// test suite metrics
	suiteErrorCounter := createIntCounter(meter, ErrorTestsCount, "Total number of failed tests")
	suiteFailedCounter := createIntCounter(meter, FailedTestsCount, "Total number of failed tests")
	suiteSkippedCounter := createIntCounter(meter, SkippedTestsCount, "Total number of skipped tests")
	suitePassedCounter := createIntCounter(meter, PassedTestsCount, "Total number of passed tests")
	suiteTestsCounter := createIntCounter(meter, TotalTestsCount, "Total number of executed tests")
	suiteDurationCounter := createIntCounter(meter, TestsDuration, "Duration of the tests")
	suiteDurationHistogram := createFloat64Histogram(meter, TestsDurationHist, "Duration of the tests", "s")

	// test case metrics
	caseFailedCounter := createIntCounter(meter, CaseFailedCount, "Total number of failed tests")
	caseErrorCounter := createIntCounter(meter, CaseErrorCount, "Total number of error tests")
	casePassedCounter := createIntCounter(meter, CasePassedCount, "Total number of passed tests")
	caseSkippedCounter := createIntCounter(meter, CaseSkippedCount, "Total number of skipped tests")
	caseDurationCounter := createIntCounter(meter, CaseDuration, "Duration of the tests")
	caseDurationHistogram := createFloat64Histogram(meter, CaseDurationHist, "Duration of the tests", "s")

	// outer span for the whole report
	ctx, outerSpan := tracer.Start(ctx, traceNameFlag, trace.WithAttributes(runtimeAttributes...),
		trace.WithSpanKind(trace.SpanKindServer))
	defer outerSpan.End()

	for _, suite := range suites {
		totals := suite.Totals

		// attributes for the suite that are common for metrics and spans
		suiteAttributes := []attribute.KeyValue{
			semconv.CodeNamespaceKey.String(suite.Package),
			attribute.Key(TestsSuiteName).String(suite.Name),
		}
		suiteAttributes = append(suiteAttributes, runtimeAttributes...)
		suiteAttributes = append(suiteAttributes, propsToLabels(suite.Properties)...)

		metricAttributes := metric.WithAttributes(suiteAttributes...)

		// metrics for the suite
		suiteErrorCounter.Add(ctx, int64(totals.Error), metricAttributes)
		suiteFailedCounter.Add(ctx, int64(totals.Failed), metricAttributes)
		suiteSkippedCounter.Add(ctx, int64(totals.Skipped), metricAttributes)
		suitePassedCounter.Add(ctx, int64(totals.Passed), metricAttributes)
		suiteTestsCounter.Add(ctx, int64(totals.Tests), metricAttributes)

		suiteDurationCounter.Add(ctx, totals.Duration.Milliseconds(), metricAttributes)
		suiteDurationHistogram.Record(ctx, totals.Duration.Seconds(), metricAttributes)

		// attributes for the suite span
		suiteSpanAttributes := append([]attribute.KeyValue{}, suiteAttributes...)
		suiteSpanAttributes = append(suiteSpanAttributes,
			attribute.Key(TestsDuration).Int64(suite.Totals.Duration.Milliseconds()),
			attribute.Key(TestsSystemErr).String(suite.SystemErr),
			attribute.Key(TestsSystemOut).String(suite.SystemOut),
		)

		// start the suite span
		ctx, suiteSpan := tracer.Start(ctx, suite.Name, trace.WithAttributes(suiteSpanAttributes...))

		// iterate tests and add metrics and spans
		for _, test := range suite.Tests {
			// attributes for the test case that are common for metrics and spans
			testAttributes := []attribute.KeyValue{
				semconv.CodeFunctionKey.String(test.Name),
				attribute.Key(TestClassName).String(test.Classname),
			}
			testAttributes = append(testAttributes, suiteAttributes...)
			testAttributes = append(testAttributes, runtimeAttributes...)
			testAttributes = append(testAttributes, propsToLabels(test.Properties)...)

			metricAttributes := metric.WithAttributes(testAttributes...)

			// metrics for the test case
			if test.Status == junit.StatusError {
				caseErrorCounter.Add(ctx, 1, metricAttributes)
			} else if test.Status == junit.StatusFailed {
				caseFailedCounter.Add(ctx, 1, metricAttributes)
			} else if test.Status == junit.StatusPassed {
				casePassedCounter.Add(ctx, 1, metricAttributes)
			} else if test.Status == junit.StatusSkipped {
				caseSkippedCounter.Add(ctx, 1, metricAttributes)
			}

			caseDurationCounter.Add(ctx, test.Duration.Milliseconds(), metricAttributes)
			caseDurationHistogram.Record(ctx, test.Duration.Seconds(), metricAttributes)

			// attributes for the test span
			testSpanAttributes := append([]attribute.KeyValue{}, testAttributes...)
			testSpanAttributes = append(testSpanAttributes,
				attribute.Key(TestDuration).Int64(test.Duration.Milliseconds()),
				attribute.Key(TestMessage).String(test.Message),
				attribute.Key(TestStatus).String(string(test.Status)),
				attribute.Key(TestSystemErr).String(test.SystemErr),
				attribute.Key(TestSystemOut).String(test.SystemOut),
			)

			if test.Error != nil {
				testSpanAttributes = append(testSpanAttributes, attribute.Key(TestError).String(test.Error.Error()))
			}

			_, testSpan := tracer.Start(ctx, test.Name, trace.WithAttributes(testSpanAttributes...))
			testSpan.End()
		}

		suiteSpan.End()
	}

	return nil
}

// getDefaultwd retrieves the current working dir, using '.' in the case an error occurs
func getDefaultwd() string {
	workingDir, err := os.Getwd()
	if err != nil {
		return "."
	}

	return workingDir
}

// getOtlpEnvVar the precedence order is: flag > env var > fallback
func getOtlpEnvVar(flag string, envVarKey string, fallback string) string {
	if flag != "" {
		return flag
	}

	envVar := os.Getenv(envVarKey)
	if envVar != "" {
		return envVar
	}

	return fallback
}

// getOtlpServiceName checks the service name
func getOtlpServiceName() string {
	return getOtlpEnvVar(serviceNameFlag, "OTEL_SERVICE_NAME", Junit2otlp)
}

// getOtlpServiceVersion checks the service version
func getOtlpServiceVersion() string {
	return getOtlpEnvVar(serviceVersionFlag, "OTEL_SERVICE_VERSION", "")
}

func initMetricsProvider(ctx context.Context, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	exporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create the collector exporter: %v", err)
	}

	reader := sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(2*time.Second))
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(meterProvider)

	return meterProvider, nil
}

func initTracerProvider(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(
			sdktrace.NewBatchSpanProcessor(
				traceExporter,
				sdktrace.WithMaxExportBatchSize(batchSizeFlag),
			),
		),
	)

	otel.SetTracerProvider(tracerProvider)

	return tracerProvider, nil
}

func propsToLabels(props map[string]string) []attribute.KeyValue {
	attributes := []attribute.KeyValue{}
	for k, v := range props {
		// if propertiesAllowedString is not "all" (default) and the key is not in the
		// allowed list, skip it
		if propertiesAllowedString != propertiesAllowAll &&
			len(propsAllowed) > 0 && !slices.Contains(propsAllowed, k) {
			continue
		}

		attributes = append(attributes, attribute.Key(k).String(v))
	}

	return attributes
}

type InputReader interface {
	Read() ([]byte, error)
}

type PipeReader struct{}

func (pr *PipeReader) Read() ([]byte, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}

	if (stat.Mode() & os.ModeCharDevice) == 0 {
		var buf []byte
		scanner := bufio.NewScanner(os.Stdin)

		// 64KB initial buffer, 1MB max buffer size
		// was seeing large failure messages causing parsing to fail
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

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

	ctx = initOtelContext(ctx)

	// set the service name that will show up in tracing UIs
	resAttrs := resource.WithAttributes(
		semconv.ServiceNameKey.String(otlpSrvName),
		semconv.ServiceVersionKey.String(otlpSrvVersion),
	)
	res, err := resource.New(ctx, resource.WithProcess(), resAttrs)
	if err != nil {
		return fmt.Errorf("failed to create OpenTelemetry service name resource: %s", err)
	}

	tracesProvides, err := initTracerProvider(ctx, res)
	if err != nil {
		return err
	}
	defer tracesProvides.Shutdown(ctx)

	provider, err := initMetricsProvider(ctx, res)
	if err != nil {
		return fmt.Errorf("failed to initialise pusher: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, time.Second*30)
		defer cancel()
		// pushes any last exports to the receiver
		if err := provider.Shutdown(ctx); err != nil {
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
