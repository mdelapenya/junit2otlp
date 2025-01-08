package transform

import (
	"context"

	"github.com/joshdk/go-junit"
	"github.com/mdelapenya/junit2otlp/internal/config"
	"github.com/mdelapenya/junit2otlp/internal/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

type OtelProvider interface {
	Tracer(string) trace.Tracer
	Meter(string) metric.Meter
}

func TransformAndLoadSuites(ctx context.Context, cfg *config.Config, provider OtelProvider,
	suites []junit.Suite, runtimeAttributes []attribute.KeyValue) error {
	ctx = otel.InitOtelContext(ctx)

	tracer := provider.Tracer(cfg.ServiceName)
	meter := provider.Meter(cfg.ServiceName)

	durationCounter := createIntCounter(meter, otel.TestsDuration, "Duration of the tests")
	errorCounter := createIntCounter(meter, otel.ErrorTestsCount, "Total number of failed tests")
	failedCounter := createIntCounter(meter, otel.FailedTestsCount, "Total number of failed tests")
	passedCounter := createIntCounter(meter, otel.PassedTestsCount, "Total number of passed tests")
	skippedCounter := createIntCounter(meter, otel.SkippedTestsCount, "Total number of skipped tests")
	testsCounter := createIntCounter(meter, otel.TotalTestsCount, "Total number of executed tests")

	ctx, outerSpan := tracer.Start(ctx, cfg.TraceName, trace.WithAttributes(runtimeAttributes...), trace.WithSpanKind(trace.SpanKindServer))
	defer outerSpan.End()

	for _, suite := range suites {
		totals := suite.Totals

		suiteAttributes := []attribute.KeyValue{
			semconv.CodeNamespaceKey.String(suite.Package),
			attribute.Key(otel.TestsSuiteName).String(suite.Name),
			attribute.Key(otel.TestsSystemErr).String(suite.SystemErr),
			attribute.Key(otel.TestsSystemOut).String(suite.SystemOut),
			attribute.Key(otel.TestsDuration).Int64(suite.Totals.Duration.Milliseconds()),
		}

		suiteAttributes = append(suiteAttributes, runtimeAttributes...)
		suiteAttributes = append(suiteAttributes, propsToLabels(cfg, suite.Properties)...)

		attributeSet := attribute.NewSet(suiteAttributes...)
		metricAttributes := metric.WithAttributeSet(attributeSet)

		durationCounter.Add(ctx, totals.Duration.Milliseconds(), metricAttributes)
		errorCounter.Add(ctx, int64(totals.Error), metricAttributes)
		failedCounter.Add(ctx, int64(totals.Failed), metricAttributes)
		passedCounter.Add(ctx, int64(totals.Passed), metricAttributes)
		skippedCounter.Add(ctx, int64(totals.Skipped), metricAttributes)
		testsCounter.Add(ctx, int64(totals.Tests), metricAttributes)

		ctx, suiteSpan := tracer.Start(ctx, suite.Name, trace.WithAttributes(suiteAttributes...))
		for _, test := range suite.Tests {
			testAttributes := []attribute.KeyValue{
				semconv.CodeFunctionKey.String(test.Name),
				attribute.Key(otel.TestDuration).Int64(test.Duration.Milliseconds()),
				attribute.Key(otel.TestClassName).String(test.Classname),
				attribute.Key(otel.TestMessage).String(test.Message),
				attribute.Key(otel.TestStatus).String(string(test.Status)),
				attribute.Key(otel.TestSystemErr).String(test.SystemErr),
				attribute.Key(otel.TestSystemOut).String(test.SystemOut),
			}

			testAttributes = append(testAttributes, propsToLabels(cfg, test.Properties)...)
			testAttributes = append(testAttributes, suiteAttributes...)

			if test.Error != nil {
				testAttributes = append(testAttributes, attribute.Key(otel.TestError).String(test.Error.Error()))
			}

			_, testSpan := tracer.Start(ctx, test.Name, trace.WithAttributes(testAttributes...))
			testSpan.End()
		}

		suiteSpan.End()
	}

	return nil
}

func propsToLabels(cfg *config.Config, props map[string]string) []attribute.KeyValue {
	attributes := []attribute.KeyValue{}
	for k, v := range props {
		if !cfg.IsPropertyAllowed(k) {
			continue
		}

		attributes = append(attributes, attribute.Key(k).String(v))
	}

	return attributes
}

func createIntCounter(meter metric.Meter, name string, description string) metric.Int64Counter {
	counter, _ := meter.Int64Counter(name, metric.WithDescription(description))
	// Accumulators always return nil errors
	// see https://github.com/open-telemetry/opentelemetry-go/blob/e8fbfd3ec52d8153eea3f13465b7de15cd8f6320/sdk/metric/sdk.go#L256-L264
	return counter
}
