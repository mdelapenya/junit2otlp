package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joshdk/go-junit"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
)

func createIntCounter(meter metric.Meter, name string, description string) metric.Int64Counter {
	return metric.Must(meter).
		NewInt64Counter(
			name,
			metric.WithDescription(description),
		)
}

func initMetricsPusher(ctx context.Context) (*controller.Controller, error) {
	client := otlpmetricgrpc.NewClient(otlpmetricgrpc.WithInsecure())
	exp, err := otlpmetric.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create the collector exporter: %v", err)
	}

	defer func() {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if err := exp.Shutdown(ctx); err != nil {
			otel.Handle(err)
		}
	}()

	pusher := controller.New(
		processor.NewFactory(
			simple.NewWithExactDistribution(),
			exp,
		),
		controller.WithExporter(exp),
		controller.WithCollectPeriod(2*time.Second),
	)
	global.SetMeterProvider(pusher)

	if err := pusher.Start(ctx); err != nil {
		return nil, fmt.Errorf("could not start metric controller: %v", err)
	}

	return pusher, nil
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
	pusher, err := initMetricsPusher(ctx)
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

	meter := global.Meter("jUnit")

	durationCounter := createIntCounter(meter, TestsDuration, "Duration of the tests")
	errorCounter := createIntCounter(meter, ErrorTestsCount, "Total number of failed tests")
	failedCounter := createIntCounter(meter, FailedTestsCount, "Total number of failed tests")
	passedCounter := createIntCounter(meter, PassedTestsCount, "Total number of passed tests")
	skippedCounter := createIntCounter(meter, SkippedTestsCount, "Total number of skipped tests")
	testsCounter := createIntCounter(meter, TotalTestsCount, "Total number of executed tests")

	xmlBuffer, err := readFromPipe()
	if err != nil {
		return fmt.Errorf("failed to read from pipe: %v", err)
	}

	suites, err := junit.Ingest(xmlBuffer)
	if err != nil {
		return fmt.Errorf("failed to ingest JUnit xml: %v", err)
	}

	for _, suite := range suites {
		totals := suite.Totals

		suiteName := attribute.KeyValue{
			Key:   attribute.Key(TestsName),
			Value: attribute.StringValue(suite.Name),
		}
		suitePackage := attribute.KeyValue{
			Key:   attribute.Key(TestsPackage),
			Value: attribute.StringValue(suite.Package),
		}

		labels := []attribute.KeyValue{suiteName, suitePackage}

		durationCounter.Add(ctx, int64(totals.Duration.Milliseconds()), labels...)
		errorCounter.Add(ctx, int64(totals.Error), labels...)
		failedCounter.Add(ctx, int64(totals.Failed), labels...)
		passedCounter.Add(ctx, int64(totals.Passed), labels...)
		skippedCounter.Add(ctx, int64(totals.Skipped), labels...)
		testsCounter.Add(ctx, int64(totals.Tests), labels...)
	}

	return nil
}

func main() {
	if err := Main(context.Background()); err != nil {
		log.Fatal(err)
	}
}
