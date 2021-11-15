package main

import (
	"bufio"
	"context"
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
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

var runtimeAttributes []attribute.KeyValue

func init() {
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

func createTracesAndSpans(ctx context.Context, tracesProvides *sdktrace.TracerProvider, suites []junit.Suite) error {
	tracer := tracesProvides.Tracer("junit2otlp")
	meter := global.Meter("junit2otlp")

	durationCounter := createIntCounter(meter, TestsDuration, "Duration of the tests")
	errorCounter := createIntCounter(meter, ErrorTestsCount, "Total number of failed tests")
	failedCounter := createIntCounter(meter, FailedTestsCount, "Total number of failed tests")
	passedCounter := createIntCounter(meter, PassedTestsCount, "Total number of passed tests")
	skippedCounter := createIntCounter(meter, SkippedTestsCount, "Total number of skipped tests")
	testsCounter := createIntCounter(meter, TotalTestsCount, "Total number of executed tests")

	ctx, outerSpan := tracer.Start(ctx, "junit2otlp", trace.WithAttributes(runtimeAttributes...))
	defer outerSpan.End()

	for _, suite := range suites {
		totals := suite.Totals

		suiteAttributes := []attribute.KeyValue{
			semconv.CodeNamespaceKey.String(suite.Package),
			attribute.Key("code.testsuite").String(suite.Name),
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
				testAttributes = append(testAttributes, attribute.Key("tests.error").String(test.Error.Error()))
			}

			_, testSpan := tracer.Start(ctx, test.Name,
				trace.WithAttributes(testAttributes...))
			testSpan.End()
		}

		suiteSpan.End()
	}

	return nil
}

func initMetricsExporter(ctx context.Context) (*otlpmetric.Exporter, error) {
	exp, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create the collector exporter: %v", err)
	}

	return exp, nil
}

func initMetricsPusher(ctx context.Context, exporter *otlpmetric.Exporter) (*controller.Controller, error) {
	pusher := controller.New(
		processor.NewFactory(
			simple.NewWithExactDistribution(),
			exporter,
		),
		controller.WithExporter(exporter),
		controller.WithCollectPeriod(2*time.Second),
	)
	global.SetMeterProvider(pusher)

	if err := pusher.Start(ctx); err != nil {
		return nil, fmt.Errorf("could not start metric controller: %v", err)
	}

	return pusher, nil
}

func initTracerProvider(ctx context.Context) (*sdktrace.TracerProvider, error) {
	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}
	tracerProvider := sdktrace.NewTracerProvider(
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

func readFromPipe() ([]byte, error) {
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

func Main(ctx context.Context) error {
	tracesProvides, err := initTracerProvider(ctx)
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

	pusher, err := initMetricsPusher(ctx, metricsExporter)
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

	xmlBuffer, err := readFromPipe()
	if err != nil {
		return fmt.Errorf("failed to read from pipe: %v", err)
	}

	suites, err := junit.Ingest(xmlBuffer)
	if err != nil {
		return fmt.Errorf("failed to ingest JUnit xml: %v", err)
	}

	return createTracesAndSpans(ctx, tracesProvides, suites)
}

func main() {
	if err := Main(context.Background()); err != nil {
		log.Fatal(err)
	}
}
