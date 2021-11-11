module github.com/mdelapenya/junit2otlp

go 1.16

require (
	github.com/joshdk/go-junit v0.0.0-20210226021600-6145f504ca0d
	go.opentelemetry.io/otel v1.1.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric v0.24.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v0.24.0
	go.opentelemetry.io/otel/metric v0.24.0
	go.opentelemetry.io/otel/sdk/metric v0.24.0
)
