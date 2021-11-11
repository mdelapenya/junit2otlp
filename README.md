# junit2otlp

This simple CLI, written in Go, is sending jUnit metrics to a back-end using [Open Telemetry](https://opentelemetry.io).

## Background
As jUnit represents a de-facto standard for test results in every programming language, this tool consumes the XML files produced by the test runner (or a tool converting to xUnit format), sending metrics to one or more open-source or commercial back-ends with Open Telemetry.

## How to use it

```shell
# Build the binary
go build
# Use the sample XML file and pass it to the binary
cat TEST-sample.xml | junit2otlp
```
