# junit2otlp

This simple CLI, written in Go, is sending jUnit metrics to a back-end using [Open Telemetry](https://opentelemetry.io).

> Inspired by https://github.com/axw/test2otlp, which sends traces and spans for `go test` JSON events as they occur.

## Background
As jUnit represents a de-facto standard for test results in every programming language, this tool consumes the XML files produced by the test runner (or a tool converting to xUnit format), sending metrics to one or more open-source or commercial back-ends with Open Telemetry.

## How to use it

```shell
# Run back-end for storing traces and spans (using Elastic Stack for this purpose)
docker-compose up -d
# Set the environment with the OTLP endpoint
eval $(cat test.env)
# Use the sample XML file and pass it to the binary
cat TEST-sample.xml | go run *.go
```
