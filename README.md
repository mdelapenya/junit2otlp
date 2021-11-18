# junit2otlp

This simple CLI, written in Go, is sending jUnit metrics to a back-end using [Open Telemetry](https://opentelemetry.io).

> Inspired by https://github.com/axw/test2otlp, which sends traces and spans for `go test` JSON events as they occur.

## Background
As jUnit represents a de-facto standard for test results in every programming language, this tool consumes the XML files produced by the test runner (or a tool converting to xUnit format), sending metrics to one or more open-source or commercial back-ends with Open Telemetry.

## Demos
To demonstrate how traces and metrics are sent to different back-ends, we are provising the following demos:

- Elastic

### Elastic
It will use the Elastic Stack as back-end, sending the traces, spans and metrics through the APM Server, storing them in Elasticsearch and finally using Kibana as visualisation layer.

```shell
make demo-start-elastic
cat TEST-sample.xml | go run main.go semconv.go
cat TEST-sample2.xml | go run main.go semconv.go
cat TEST-sample3.xml | go run main.go semconv.go
open http://localhost:5601/app/apm/services?rangeFrom=now-15m&rangeTo=now&comparisonEnabled=true&comparisonType=day
```

### Jaeger
It will use Jaeger as back-end, sending the traces, spans and metrics through the OpenTelemetry collector, storing them in memory.

```shell
make demo-start-jaeger
cat TEST-sample.xml | go run main.go semconv.go
cat TEST-sample2.xml | go run main.go semconv.go
cat TEST-sample3.xml | go run main.go semconv.go
open http://localhost:16686
```
