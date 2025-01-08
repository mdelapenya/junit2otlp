package otel

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mdelapenya/junit2otlp/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

type OtelProvider struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
}

func (p *OtelProvider) Handle(err error) {
	otel.Handle(err)
}

func (p *OtelProvider) Tracer(serviceName string) trace.Tracer {
	return p.TracerProvider.Tracer(serviceName)
}

func (p *OtelProvider) Meter(serviceName string) metric.Meter {
	return p.MeterProvider.Meter(serviceName)
}

func (p *OtelProvider) Shutdown(ctx context.Context) {
	err := p.TracerProvider.Shutdown(ctx)
	if err != nil {
		log.Printf("failed to shutdown tracer provider: %v", err)
		otel.Handle(err)
	}

	err = p.MeterProvider.Shutdown(ctx)
	if err != nil {
		log.Printf("failed to shutdown meter provider: %v", err)
		otel.Handle(err)
	}
}

func NewProvider(ctx context.Context, cfg *config.Config) (*OtelProvider, error) {
	// set the service name that will show up in tracing UIs
	resAttrs := resource.WithAttributes(
		semconv.ServiceNameKey.String(cfg.ServiceName),
		semconv.ServiceVersionKey.String(cfg.ServiceVersion),
	)
	res, err := resource.New(ctx, resource.WithProcess(), resAttrs)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenTelemetry service name resource: %s", err)
	}

	tracesProvides, err := initTracerProvider(ctx, cfg, res)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize trace pusher: %v", err)
	}

	provider, err := initMetricsProvider(ctx, cfg, res)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize metric pusher: %v", err)
	}

	return &OtelProvider{
		TracerProvider: tracesProvides,
		MeterProvider:  provider,
	}, nil
}

func initMetricsProvider(ctx context.Context, cfg *config.Config, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	if cfg.SkipMetrics {
		return sdkmetric.NewMeterProvider(), nil
	}

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

func initTracerProvider(ctx context.Context, cfg *config.Config, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	if cfg.SkipTraces {
		return sdktrace.NewTracerProvider(), nil
	}

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(
			sdktrace.NewBatchSpanProcessor(
				traceExporter,
				sdktrace.WithMaxExportBatchSize(cfg.BatchSize),
			),
		),
	)

	otel.SetTracerProvider(tracerProvider)

	return tracerProvider, nil
}
