package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
)

const exporterEndpointKey = "OTEL_EXPORTER_OTLP_ENDPOINT"

var originalEnvVar string
var originalServiceNameFlag string
var originalEndpoint string

func init() {
	originalEndpoint = os.Getenv(exporterEndpointKey)
	originalEnvVar = os.Getenv("OTEL_SERVICE_NAME")
	originalServiceNameFlag = serviceNameFlag
}

type TestReader struct {
	testFile string
}

func (tr *TestReader) Read() ([]byte, error) {
	b, err := ioutil.ReadFile(tr.testFile)
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
	assert.Equal(t, expected, att.StringValue)
}

func findAttributeInArray(attributes []TestAttribute, key string) (TestAttribute, error) {
	for _, att := range attributes {
		if att.Key == key {
			return att, nil
		}
	}

	return TestAttribute{}, fmt.Errorf("attribute with key '%s' not found", key)
}

func Test_Main_SampleXML(t *testing.T) {
	os.Setenv("BRANCH", "main")
	defer func() {
		os.Unsetenv("BRANCH")
	}()

	ctx := context.Background()

	// create file for otel to store the traces
	workingDir, err := os.Getwd()
	if err != nil {
		t.Error()
	}

	buildDir := path.Join(workingDir, "build")
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		err = os.Mkdir(buildDir, 0755)
		if err != nil {
			t.Error(err)
		}
	}

	reportFilePath := path.Join(buildDir, "otel-collector.json")
	reportFile, err := os.Create(reportFilePath)
	if err != nil {
		t.Error(err)
	}
	defer reportFile.Close()

	// create docker network for the containers
	networkName := "jaeger-integration-tests"

	network, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name: networkName,
		},
	})
	if err != nil {
		t.Error(err)
	}

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
	if err != nil {
		t.Error(err)
	}

	otelCollector, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "otel/opentelemetry-collector-contrib-dev:93a9885459c9406db8ac446f77f290b02542e8d5",
			ExposedPorts: []string{
				"1888/tcp",  // pprof extension
				"13133/tcp", // health_check extension
				"4317/tcp",  // OTLP gRPC receiver
				"55679/tcp", // zpages extension
			},
			BindMounts: map[string]string{
				path.Join(workingDir, "testresources", "otel-collector-config.yml"): "/etc/otel/config.yaml",
				reportFilePath: "/tmp/tests.json",
			},
		},
		Started: true,
	})
	if err != nil {
		t.Error(err)
	}

	collectorPort, err := otelCollector.MappedPort(ctx, "4317/tcp")
	if err != nil {
		t.Errorf("could not get mapped port for otel-collector: %v", err)
	}

	os.Setenv(exporterEndpointKey, "http://localhost:"+collectorPort.Port())
	os.Setenv("OTEL_EXPORTER_OTLP_SPAN_INSECURE", "true")
	os.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	os.Setenv("OTEL_EXPORTER_OTLP_METRIC_INSECURE", "true")
	os.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "")
	os.Setenv("OTEL_SERVICE_NAME", "jaeger-srv-test")

	defer func() {
		err := otelCollector.Terminate(ctx)
		if err != nil {
			t.Error(err)
		}
		err = jaeger.Terminate(ctx)
		if err != nil {
			t.Error(err)
		}
		// clean up network
		err = network.Remove(ctx)
		if err != nil {
			t.Error(err)
		}

		// clean up test report
		os.Remove(reportFilePath)

		// reset environment
		os.Setenv(exporterEndpointKey, originalEndpoint)
	}()

	err = Main(context.Background(), &TestReader{testFile: "TEST-sample.xml"})
	if err != nil {
		t.Error()
	}

	// TODO: retry until the file is written by the otel-exporter
	time.Sleep(time.Second * 30)

	// assert using the generated file
	jsonBytes, _ := ioutil.ReadFile(reportFilePath)

	// merge both JSON files
	// 1. get the spans and metrics JSONs, they are separated by \n
	// 2. remote white spaces
	// 3. unmarshal each resource separately
	// 4. assign each resource to the test report struct
	content := string(jsonBytes)
	jsons := strings.Split(content, "\n")
	jsonSpans := strings.TrimSpace(jsons[0])
	jsonMetrics := strings.TrimSpace(jsons[1])

	var resSpans ResourceSpans
	err = json.Unmarshal([]byte(jsonSpans), &resSpans)
	if err != nil {
		t.Error(err.Error())
	}
	var resMetrics ResourceMetrics
	err = json.Unmarshal([]byte(jsonMetrics), &resMetrics)
	if err != nil {
		t.Error(err.Error())
	}

	testReport := TestReport{
		resourceSpans:   resSpans,
		resourceMetrics: resMetrics,
	}

	resourceSpans := testReport.resourceSpans.Spans[0]

	srvNameAttribute, _ := findAttributeInArray(resourceSpans.Resource.Attributes, "service.name")
	assert.Equal(t, "service.name", srvNameAttribute.Key)
	assertStringValueInAttribute(t, srvNameAttribute.Value, "jaeger-srv-test")

	srvVersionAttribute, _ := findAttributeInArray(resourceSpans.Resource.Attributes, "service.version")
	assert.Equal(t, "service.version", srvVersionAttribute.Key)
	assertStringValueInAttribute(t, srvVersionAttribute.Value, "")

	instrumentationLibrarySpans := resourceSpans.InstrumentationLibrarySpans[0]

	assert.Equal(t, "jaeger-srv-test", instrumentationLibrarySpans.InstrumentationLibrary.Name)

	spans := instrumentationLibrarySpans.Spans

	// there are 15 elements:
	//   1 testsuites element (root element)
	// 	 3 testsuite element
	// 	 11 testcase elements
	expectedSpansCount := 15

	assert.Equal(t, expectedSpansCount, len(spans))

	aTestCase := spans[2]
	assert.Equal(t, "TestCheckConfigDirsCreatesWorkspaceAtHome", aTestCase.Name)
	assert.Equal(t, "SPAN_KIND_INTERNAL", aTestCase.Kind)

	codeFunction, _ := findAttributeInArray(aTestCase.Attributes, "code.function")
	assertStringValueInAttribute(t, codeFunction.Value, "TestCheckConfigDirsCreatesWorkspaceAtHome")

	testClassName, _ := findAttributeInArray(aTestCase.Attributes, "tests.case.classname")
	assertStringValueInAttribute(t, testClassName.Value, "github.com/elastic/e2e-testing/cli/config")

	goVersion, _ := findAttributeInArray(aTestCase.Attributes, "go.version")
	assertStringValueInAttribute(t, goVersion.Value, "go1.16.3 linux/amd64")

	// last span is server type
	aTestCase = spans[expectedSpansCount-1]
	assert.Equal(t, "SPAN_KIND_SERVER", aTestCase.Kind)
}

func Test_GetServiceVariable(t *testing.T) {
	var otlpTests = []struct {
		fallback     string
		getFn        func() string
		setFlag      func()
		otelVariable string
	}{
		{
			fallback: Junit2otlp,
			getFn:    getOtlpServiceName,
			setFlag: func() {
				serviceNameFlag = "this-is-a-flag"
			},
			otelVariable: "OTEL_SERVICE_NAME",
		},
		{
			fallback: "",
			getFn:    getOtlpServiceVersion,
			setFlag: func() {
				serviceVersionFlag = "this-is-a-flag"
			},
			otelVariable: "OTEL_SERVICE_VERSION",
		},
	}

	for _, otlpotlpTest := range otlpTests {
		t.Run("Read "+otlpotlpTest.otelVariable+" from environment", func(t *testing.T) {
			t.Run("Without environment variable retrieves fallback", func(t *testing.T) {
				os.Unsetenv(otlpotlpTest.otelVariable)
				defer resetEnvironment(otlpotlpTest.otelVariable)

				actualValue := otlpotlpTest.getFn()

				assert.Equal(t, otlpotlpTest.fallback, actualValue)
			})

			t.Run("With environment variable retrieves the variable", func(t *testing.T) {
				os.Setenv(otlpotlpTest.otelVariable, "foobar")
				defer resetEnvironment(otlpotlpTest.otelVariable)

				actualValue := otlpotlpTest.getFn()

				assert.Equal(t, "foobar", actualValue)
			})
		})

		t.Run("Read "+otlpotlpTest.otelVariable+" from command line flag", func(t *testing.T) {
			t.Run("Without environment variable retrieves the flag", func(t *testing.T) {
				os.Unsetenv(otlpotlpTest.otelVariable)
				otlpotlpTest.setFlag()
				defer resetEnvironment(otlpotlpTest.otelVariable)

				actualValue := otlpotlpTest.getFn()

				assert.Equal(t, "this-is-a-flag", actualValue)
			})

			t.Run("With environment variable retrieves the variable", func(t *testing.T) {
				os.Setenv(otlpotlpTest.otelVariable, "foobar")
				otlpotlpTest.setFlag()
				defer resetEnvironment(otlpotlpTest.otelVariable)

				actualValue := otlpotlpTest.getFn()

				assert.Equal(t, "foobar", actualValue)
				assert.Equal(t, "foobar", os.Getenv(otlpotlpTest.otelVariable))
			})
		})
	}
}

func resetEnvironment(otelVariable string) {
	os.Setenv(otelVariable, originalEnvVar)

	serviceNameFlag = originalServiceNameFlag
}
