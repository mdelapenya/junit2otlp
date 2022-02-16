module github.com/mdelapenya/junit2otlp

go 1.16

require (
	github.com/go-git/go-git/v5 v5.4.2
	github.com/joshdk/go-junit v0.0.0-20210226021600-6145f504ca0d
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.7.0
	github.com/testcontainers/testcontainers-go v0.11.1
	go.opentelemetry.io/otel v1.4.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric v0.27.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v0.27.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.4.0
	go.opentelemetry.io/otel/metric v0.27.0
	go.opentelemetry.io/otel/sdk v1.4.0
	go.opentelemetry.io/otel/sdk/metric v0.27.0
	go.opentelemetry.io/otel/trace v1.4.0
)
