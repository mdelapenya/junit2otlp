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
| Trace Name | --trace-name | `junit2otlp` | Overrides OpenTelemetry's trace name. |

For further reference on environment variables in the OpenTelemetry SDK, please read the [official specification](https://opentelemetry.io/docs/reference/specification/sdk-environment-variables/)

## Docker image
It's possible to run the binary as a Docker image. To build and use the image

1. First build the Docker image using this Make goal:
```shell
make buildDockerImage
```

2. Then start the Elastic Stack back-end:
```shell
make demo-start-elastic
```

3. Finally, once the services are started, run:
```
cat TEST-sample3.xml | docker run --rm -i --network elastic_junit2otlp --env OTEL_EXPORTER_OTLP_ENDPOINT=http://apm-server:8200 mdelapenya/junit2otlp:latest --service-name DOCKERFOO --trace-name TRACEBAR
```
  - We are making the Docker container receive the pipe with the `-i` flag.
  - We are attaching the container to the same Docker network where the services are running.
  - We are passing an environment variable with the URL of the OpenTelemetry exporter endpoint, in this case an APM Server instance.
  - We are passing command line flags to the container, setting the service name (_DOCKERFOO_) and the trace name (_TRACEBAR_).

## Demos
To demonstrate how traces and metrics are sent to different back-ends, we are provising the following demos:

- Elastic
- Jaeger
- Prometheus
- Zipkin

### Elastic
It will use the Elastic Stack as back-end, sending the traces, spans and metrics through the APM Server, storing them in Elasticsearch and finally using Kibana as visualisation layer.

```shell
make demo-start-elastic
go build && chmod +x ./junit2otlp
cat TEST-sample.xml | ./junit2otlp
cat TEST-sample2.xml | ./junit2otlp
cat TEST-sample3.xml | ./junit2otlp
open http://localhost:5601/app/apm/services?rangeFrom=now-15m&rangeTo=now&comparisonEnabled=true&comparisonType=day
```

### Jaeger
It will use Jaeger as back-end, sending the traces, spans and metrics through the OpenTelemetry collector, storing them in memory.

```shell
make demo-start-jaeger
go build && chmod +x ./junit2otlp
cat TEST-sample.xml | ./junit2otlp
cat TEST-sample2.xml | ./junit2otlp
cat TEST-sample3.xml | ./junit2otlp
open http://localhost:16686
```

### Prometheus
It will use Prometheus as back-end, sending the traces, spans and metrics through the OpenTelemetry collector, storing them in memory.

```shell
make demo-start-prometheus
go build && chmod +x ./junit2otlp
cat TEST-sample.xml | ./junit2otlp
cat TEST-sample2.xml | ./junit2otlp
cat TEST-sample3.xml | ./junit2otlp
open http://localhost:9090
```

### Zipkin
It will use Prometheus as back-end, sending the traces, spans and metrics through the OpenTelemetry collector, storing them in memory.

```shell
make demo-start-zipkin
go build && chmod +x ./junit2otlp
cat TEST-sample.xml | ./junit2otlp
cat TEST-sample2.xml | ./junit2otlp
cat TEST-sample3.xml | ./junit2otlp
open http://localhost:9411
```
