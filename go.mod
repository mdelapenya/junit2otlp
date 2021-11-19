module github.com/mdelapenya/junit2otlp

go 1.16

require (
	github.com/go-git/go-git/v5 v5.4.2
	github.com/joshdk/go-junit v0.0.0-20210226021600-6145f504ca0d
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/otel v1.1.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric v0.24.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v0.24.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.1.0
	go.opentelemetry.io/otel/metric v0.24.0
	go.opentelemetry.io/otel/sdk v1.1.0
	go.opentelemetry.io/otel/sdk/metric v0.24.0
	go.opentelemetry.io/otel/trace v1.1.0
)
