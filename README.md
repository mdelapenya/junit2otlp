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

## OpenTelemetry Attributes
This tool is going to parse the XML report produced by jUnit, or any other tool converting to that format, adding different attributes, separated by different categories:

- Test metrics attributes
- Ownership attributes

### Metrics and Traces
The following attributes are added as metrics and/or traces.

#### Test execution attributes
For each test execution, represented by a test report file, the tool will add the following attributes to the metric document, including them in the trace representing the test execution.

| Attribute | Description |
| --------- | ----------- |
| `tests.failed` | Number of failed tests in the test execution |
| `tests.error` | Number of errored tests in the test execution |
| `tests.passed` | Number of passed tests in the test execution |
| `tests.skipped` | Number of skipped tests in the test execution |
| `tests.duration` | Duration of the test execution |
| `tests.suitename` | Name of the test execution |
| `tests.systemerr` | Log produced by Systemerr |
| `tests.systemout` | Log produced by Systemout |
| `tests.total` | Total number of tests in the test execution |

#### Test case attributes
For each test case in the test execution, the tool will add the following attributes to the span document representing the test case:

| Attribute | Description |
| --------- | ----------- |
| `test.classname` | Classname or file for the test case |
| `test.duration` | Duration of the test case |
| `test.error` | Error message of the test case |
| `test.message` | Message of the test case |
| `test.status` | Status of the test case |
| `test.systemerr` | Log produced by Systemerr |
| `test.systemout` | Log produced by Systemout |

### Ownership attributes
These attributes are added to the traces and spans sent by the tool, identifying the owner (or owners) of the test suite, trying to correlate a test failure with an author or authors. To identify the owner, the tool will inspect the SCM repository for the project.

#### SCM attributes
Because the XML test report is evaluated for a project **in a SCM repository**, the tool will add the following attributes to each trace and span:

| Attribute | Description |
| --------- | ----------- |
| `scm.authors` | Array of unique Email addresses for the authors of the commits |
| `scm.baseRef` | Name of the target branch (Only for change requests) |
| `scm.branch` | Name of the branch where the test execution is processed |
| `scm.committers` | Array of unique Email addresses for the committers of the commits |
| `scm.provider` | Optional. If present, will include the name of the SCM provider, such as Github, Gitlab, Bitbucket, etc. |
| `scm.repository` | Array of unique URLs representing the repository (i.e. https://github.com/mdelapenya/junit2otlp) |
| `scm.type` | Type of the SCM (i.e. git, svn, mercurial)  At this moment the tool only supports Git repositories. |

#### Change request attributes
The tool will add the following attributes to each trace and span if and only if the XML test report is evaluated in the context of a change requests **for a Git repository**:

| Attribute | Description |
| --------- | ----------- |
| `scm.git.additions` | Number of added lines in the changeset |
| `scm.git.deletions` | Number of deleted lines in the changeset |
| `scm.git.clone.depth` | Depth of the git clone |
| `scm.git.clone.shallow` | Whethere the git clone was shallow or not |
| `scm.git.files.modified` | Number of modified files in the changeset |

A changeset is calculated based on the HEAD commit and the first ancestor between HEAD and the branch where the changeset is submitted against.

## Docker image
It's possible to run the binary as a Docker image. To build and use the image

1. First build the Docker image using this Make goal:
```shell
make build-docker-image
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
