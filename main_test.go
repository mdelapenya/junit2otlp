package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mdelapenya/junit2otlp/internal/config"
	"github.com/mdelapenya/junit2otlp/internal/readers"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const exporterEndpointKey = "OTEL_EXPORTER_OTLP_ENDPOINT"

type TestAttributeValue struct {
	IntValue    string `json:"intValue"`
	StringValue string `json:"stringValue"`
}

type TestAttribute struct {
	Key   string             `json:"key"`
	Value TestAttributeValue `json:"value,omitempty"`
}

type ResourceSpans struct {
	Spans []ResourceSpan `json:"resourceSpans"`
}
type ResourceSpan struct {
	Resource struct {
		Attributes []TestAttribute `json:"attributes"`
	} `json:"resource"`
	InstrumentationLibrarySpans []struct {
		InstrumentationLibrary struct {
			Name string `json:"name"`
		} `json:"instrumentationLibrary"`
		Spans []struct {
			TraceID           string          `json:"traceId"`
			SpanID            string          `json:"spanId"`
			ParentSpanID      string          `json:"parentSpanId"`
			Name              string          `json:"name"`
			Kind              string          `json:"kind"`
			StartTimeUnixNano string          `json:"startTimeUnixNano"`
			EndTimeUnixNano   string          `json:"endTimeUnixNano"`
			Attributes        []TestAttribute `json:"attributes"`
			Status            struct {
			} `json:"status"`
		} `json:"spans"`
	} `json:"instrumentationLibrarySpans"`
}

type ResourceMetrics struct {
	Metrics []ResourceMetric `json:"resourceMetrics"`
}
type ResourceMetric struct {
	Resource struct {
		Attributes []TestAttribute `json:"attributes"`
	} `json:"resource"`
	InstrumentationLibraryMetrics []struct {
		InstrumentationLibrary struct {
			Name string `json:"name"`
		} `json:"instrumentationLibrary"`
		Metrics []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Sum         struct {
				DataPoints []struct {
					Attributes        []TestAttribute `json:"attributes"`
					StartTimeUnixNano string          `json:"startTimeUnixNano"`
					TimeUnixNano      string          `json:"timeUnixNano"`
					AsInt             string          `json:"asInt"`
				} `json:"dataPoints"`
				AggregationTemporality string `json:"aggregationTemporality"`
				IsMonotonic            bool   `json:"isMonotonic"`
			} `json:"sum"`
		} `json:"metrics"`
	} `json:"instrumentationLibraryMetrics"`
	SchemaURL string `json:"schemaUrl"`
}

// TestReport holds references to the File exporter defined by the otel-collector
type TestReport struct {
	resourceSpans   ResourceSpans
	resourceMetrics ResourceMetrics
}

func assertStringValueInAttribute(t *testing.T, att TestAttributeValue, expected string) {
	t.Helper()

	require.Equal(t, expected, att.StringValue)
}

func requireAttributeInArray(t *testing.T, attributes []TestAttribute, key string) TestAttribute {
	t.Helper()

	for _, att := range attributes {
		if att.Key == key {
			return att
		}
	}

	t.Fatalf("attribute with key '%s' not found", key)

	return TestAttribute{}
}

func setupRuntimeDependencies(t *testing.T) (context.Context, string, testcontainers.Container) {
	ctx := context.Background()

	// create file for otel to store the traces
	tmpDir := t.TempDir()

	reportFilePath := filepath.Join(tmpDir, "otel-collector.json")
	reportFile, err := os.Create(reportFilePath)
	require.NoError(t, err)
	defer reportFile.Close()

	// create docker network for the containers
	nw, err := network.New(ctx)
	testcontainers.CleanupNetwork(t, nw)
	require.NoError(t, err)

	networkName := nw.Name

	jaeger, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "jaegertracing/all-in-one:latest",
			ExposedPorts: []string{
				"14250/tcp",
				"14268/tcp",
				"16686/tcp",
			},
			Networks: []string{networkName},
		},
		Started: true,
	})
	testcontainers.CleanupContainer(t, jaeger)
	require.NoError(t, err)

	otelCollector, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "otel/opentelemetry-collector-contrib-dev:93a9885459c9406db8ac446f77f290b02542e8d5",
			ExposedPorts: []string{
				"1888/tcp",  // pprof extension
				"13133/tcp", // health_check extension
				"4317/tcp",  // OTLP gRPC receiver
				"55679/tcp", // zpages extension
			},
			Files: []testcontainers.ContainerFile{
				{
					ContainerFilePath: "/etc/otel/config.yaml",
					HostFilePath:      filepath.Join("testdata", "otel-collector-config.yml"),
				},
				{
					Reader:            reportFile,
					ContainerFilePath: "/tmp/tests.json",
				},
			},
			WaitingFor: wait.ForListeningPort("4317/tcp"),
		},
		Started: true,
	})
	testcontainers.CleanupContainer(t, otelCollector)
	require.NoError(t, err)

	collectorPort, err := otelCollector.MappedPort(ctx, "4317/tcp")
	require.NoError(t, err)

	t.Setenv(exporterEndpointKey, "http://localhost:"+collectorPort.Port())
	t.Setenv("OTEL_EXPORTER_OTLP_SPAN_INSECURE", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_METRIC_INSECURE", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "")
	t.Setenv("OTEL_SERVICE_NAME", "jaeger-srv-test")

	return ctx, reportFilePath, otelCollector
}

func Test_Run_SampleXML(t *testing.T) {
	t.Setenv("BRANCH", "main")

	ctx, reportFilePath, otelCollector := setupRuntimeDependencies(t)
	defer func() {
		os.Remove(reportFilePath)
	}()

	cfg := config.NewConfigFromDefaults()
	cfg.ServiceName = "jaeger-srv-test"
	cfg.BatchSize = 25

	reader := readers.NewFileReader("TEST-sample.xml")

	err := Run(context.Background(), cfg, reader)
	require.NoError(t, err)

	testReport := readTestReport(t, ctx, otelCollector)

	resourceSpans := testReport.resourceSpans.Spans[0]

	srvNameAttribute := requireAttributeInArray(t, resourceSpans.Resource.Attributes, "service.name")
	require.NoError(t, err)
	require.Equal(t, "service.name", srvNameAttribute.Key)
	assertStringValueInAttribute(t, srvNameAttribute.Value, "jaeger-srv-test")

	srvVersionAttribute := requireAttributeInArray(t, resourceSpans.Resource.Attributes, "service.version")
	require.NoError(t, err)
	require.Equal(t, "service.version", srvVersionAttribute.Key)
	assertStringValueInAttribute(t, srvVersionAttribute.Value, "")

	instrumentationLibrarySpans := resourceSpans.InstrumentationLibrarySpans[0]

	require.Equal(t, "jaeger-srv-test", instrumentationLibrarySpans.InstrumentationLibrary.Name)

	spans := instrumentationLibrarySpans.Spans

	// there are 15 elements:
	//   1 testsuites element (root element)
	// 	 3 testsuite element
	// 	 11 testcase elements
	expectedSpansCount := 15

	require.Equal(t, expectedSpansCount, len(spans))

	aTestCase := spans[2]
	require.Equal(t, "TestCheckConfigDirsCreatesWorkspaceAtHome", aTestCase.Name)
	require.Equal(t, "SPAN_KIND_INTERNAL", aTestCase.Kind)

	codeFunction := requireAttributeInArray(t, aTestCase.Attributes, "code.function")
	require.NoError(t, err)
	assertStringValueInAttribute(t, codeFunction.Value, "TestCheckConfigDirsCreatesWorkspaceAtHome")

	testClassName := requireAttributeInArray(t, aTestCase.Attributes, "tests.case.classname")
	require.NoError(t, err)
	assertStringValueInAttribute(t, testClassName.Value, "github.com/elastic/e2e-testing/cli/config")

	goVersion := requireAttributeInArray(t, aTestCase.Attributes, "go.version")
	require.NoError(t, err)
	assertStringValueInAttribute(t, goVersion.Value, "go1.16.3 linux/amd64")

	// last span is server type
	aTestCase = spans[expectedSpansCount-1]
	require.Equal(t, "SPAN_KIND_SERVER", aTestCase.Kind)
}

func Test_Run_NoMetrics(t *testing.T) {
	t.Setenv("BRANCH", "main")

	ctx, reportFilePath, otelCollector := setupRuntimeDependencies(t)
	defer func() {
		os.Remove(reportFilePath)
	}()

	cfg := config.NewConfigFromDefaults()
	cfg.ServiceName = "jaeger-srv-test"
	cfg.SkipMetrics = true
	cfg.BatchSize = 25

	reader := readers.NewFileReader("TEST-sample.xml")

	err := Run(context.Background(), cfg, reader)
	require.NoError(t, err)

	testReport := readTestReport(t, ctx, otelCollector)

	resourceSpans := testReport.resourceSpans.Spans[0]
	instrumentationLibrarySpans := resourceSpans.InstrumentationLibrarySpans[0]
	spans := instrumentationLibrarySpans.Spans
	require.Equal(t, 15, len(spans))

	// there should be no metrics
	require.Empty(t, testReport.resourceMetrics.Metrics)
}

func Test_Run_NoTraces(t *testing.T) {
	t.Setenv("BRANCH", "main")

	ctx, reportFilePath, otelCollector := setupRuntimeDependencies(t)
	defer func() {
		os.Remove(reportFilePath)
	}()

	cfg := config.NewConfigFromDefaults()
	cfg.ServiceName = "jaeger-srv-test"
	cfg.SkipTraces = true
	cfg.BatchSize = 25

	reader := readers.NewFileReader("TEST-sample.xml")

	err := Run(context.Background(), cfg, reader)
	require.NoError(t, err)

	testReport := readTestReport(t, ctx, otelCollector)

	// there should be no spans
	require.Empty(t, testReport.resourceSpans.Spans)

	// there should be metrics
	require.NotEmpty(t, testReport.resourceMetrics.Metrics)
}

func readTestReport(t *testing.T, ctx context.Context, otelCollector testcontainers.Container) TestReport {
	// wait for the file to be written by the otel-exporter
	var out bytes.Buffer
	err := wait.ForFile("/tmp/tests.json").
		WithStartupTimeout(time.Second*10).
		WithPollInterval(time.Second).
		WithMatcher(func(r io.Reader) error {
			if _, err := io.Copy(&out, r); err != nil {
				return fmt.Errorf("copy: %w", err)
			}
			return nil
		}).WaitUntilReady(ctx, otelCollector)
	require.NoError(t, err)

	// assert using the generated file
	// merge both JSON files
	// 1. get the spans and metrics JSONs, they are separated by \n
	// 2. remote white spaces
	// 3. unmarshal each resource separately
	// 4. append resources to the test report struct
	testReport := TestReport{
		resourceSpans:   ResourceSpans{},
		resourceMetrics: ResourceMetrics{},
	}

	jsons := strings.Split(strings.TrimSpace(out.String()), "\n")
	for _, jsonStr := range jsons {
		if strings.Contains(jsonStr, "resourceSpans") {
			spans := ResourceSpans{}
			err = json.Unmarshal([]byte(jsonStr), &spans)
			require.NoError(t, err)

			testReport.resourceSpans.Spans = append(testReport.resourceSpans.Spans, spans.Spans...)
		} else {
			metrics := ResourceMetrics{}
			err = json.Unmarshal([]byte(jsonStr), &metrics)
			require.NoError(t, err)

			testReport.resourceMetrics.Metrics = append(testReport.resourceMetrics.Metrics, metrics.Metrics...)
		}
	}

	return testReport
}
