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

	"github.com/stretchr/testify/require"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const exporterEndpointKey = "OTEL_EXPORTER_OTLP_ENDPOINT"

type TestReader struct {
	testFile string
}

func (tr *TestReader) Read() ([]byte, error) {
	b, err := os.ReadFile(tr.testFile)
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}

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

func Test_Main_SampleXML(t *testing.T) {
	t.Setenv("BRANCH", "main")

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
		},
		Started: true,
	})
	testcontainers.CleanupContainer(t, otelCollector)
	require.NoError(t, err)

	collectorPort, err := otelCollector.MappedPort(ctx, "4317/tcp")
	if err != nil {
		t.Errorf("could not get mapped port for otel-collector: %v", err)
	}

	t.Setenv(exporterEndpointKey, "http://localhost:"+collectorPort.Port())
	t.Setenv("OTEL_EXPORTER_OTLP_SPAN_INSECURE", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_METRIC_INSECURE", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "")
	t.Setenv("OTEL_SERVICE_NAME", "jaeger-srv-test")

	defer func() {
		// clean up test report
		os.Remove(reportFilePath)
	}()

	err = Main(context.Background(), &TestReader{testFile: "TEST-sample.xml"})
	require.NoError(t, err)

	// wait for the file to be written by the otel-exporter
	var out bytes.Buffer
	err = wait.ForFile("/tmp/tests.json").
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
	// 4. assign each resource to the test report struct
	jsons := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(jsons) != 2 {
		t.Errorf("expected 2 JSONs, got %d - %s", len(jsons), jsons)
	}

	jsonSpans := ""
	jsonMetrics := ""
	// the order of the lines is not guaranteed
	if strings.Contains(jsons[0], "resourceSpans") {
		jsonSpans = strings.TrimSpace(jsons[0])
		jsonMetrics = strings.TrimSpace(jsons[1])
	} else {
		jsonSpans = strings.TrimSpace(jsons[1])
		jsonMetrics = strings.TrimSpace(jsons[0])
	}

	var resSpans ResourceSpans
	err = json.Unmarshal([]byte(jsonSpans), &resSpans)
	require.NoError(t, err)

	var resMetrics ResourceMetrics
	err = json.Unmarshal([]byte(jsonMetrics), &resMetrics)
	require.NoError(t, err)

	testReport := TestReport{
		resourceSpans:   resSpans,
		resourceMetrics: resMetrics,
	}

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

func Test_GetServiceVariable(t *testing.T) {
	var otlpTests = []struct {
		fallback     string
		getFn        func() string
		setFlag      func(string)
		otelVariable string
	}{
		{
			fallback: Junit2otlp,
			getFn:    getOtlpServiceName,
			setFlag: func(value string) {
				serviceNameFlag = value
			},
			otelVariable: "OTEL_SERVICE_NAME",
		},
		{
			fallback: "",
			getFn:    getOtlpServiceVersion,
			setFlag: func(value string) {
				serviceVersionFlag = value
			},
			otelVariable: "OTEL_SERVICE_VERSION",
		},
	}

	for _, otlpotlpTest := range otlpTests {
		t.Run(otlpotlpTest.otelVariable, func(t *testing.T) {
			t.Run("no-env/no-flag/fallback", func(t *testing.T) {
				t.Setenv(otlpotlpTest.otelVariable, "")
				otlpotlpTest.setFlag("")

				actualValue := otlpotlpTest.getFn()

				require.Equal(t, otlpotlpTest.fallback, actualValue)
			})

			t.Run("env/no-flag/env", func(t *testing.T) {
				t.Setenv(otlpotlpTest.otelVariable, "foobar")
				otlpotlpTest.setFlag("")

				actualValue := otlpotlpTest.getFn()

				require.Equal(t, "foobar", actualValue)
			})

			t.Run("no-env/flag/flag", func(t *testing.T) {
				t.Setenv(otlpotlpTest.otelVariable, "")
				otlpotlpTest.setFlag("this-is-a-flag")

				actualValue := otlpotlpTest.getFn()

				require.Equal(t, "this-is-a-flag", actualValue)
			})

			t.Run("env/flag/flag", func(t *testing.T) {
				t.Setenv(otlpotlpTest.otelVariable, "foobar")
				otlpotlpTest.setFlag("this-is-a-flag")

				actualValue := otlpotlpTest.getFn()

				require.Equal(t, "this-is-a-flag", actualValue)
			})
		})
	}
}
