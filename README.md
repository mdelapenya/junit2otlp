# junit2otlp

This simple CLI, written in Go, is sending jUnit metrics to a back-end using [Open Telemetry](https://opentelemetry.io).

> Inspired by https://github.com/axw/test2otlp, which sends traces and spans for `go test` JSON events as they occur.

## Background
As jUnit represents a de-facto standard for test results in every programming language, this tool consumes the XML files produced by the test runner (or a tool converting to xUnit format), sending metrics to one or more open-source or commercial back-ends with Open Telemetry.

## OpenTelemetry configuration
This tool is able to override the following attributes:

| Attribute | Flag | Default value | Description |
| --------- | ---- | ------------- | ----------- |
| Service Name | --service-name | `junit2otlp` | Overrides OpenTelemetry's service name. If the `OTEL_SERVICE_NAME` environment variable is set, it will take precedence over any other value. |
| Service Version | --service-version | Empty | Overrides OpenTelemetry's service version. If the `OTEL_SERVICE_VERSION` environment variable is set, it will take precedence over any other value. |

For further reference on environment variables in the OpenTelemetry SDK, please read the [official specification](https://opentelemetry.io/docs/reference/specification/sdk-environment-variables/)

## How to use it

```shell
# Run back-end for storing traces and spans (using Elastic Stack for this purpose)
docker-compose up -d
# Set the environment with the OTLP endpoint
eval $(cat test.env)
# Use the sample XML file and pass it to the binary
cat TEST-sample.xml | go run main.go semconv.go
cat TEST-sample2.xml | go run main.go semconv.go
cat TEST-sample3.xml | go run main.go semconv.go
```
