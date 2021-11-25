package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
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

type TestReader struct{}

func (tr *TestReader) Read() ([]byte, error) {
	b, err := ioutil.ReadFile("TEST-sample.xml")
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}

type TestTraceAttributeValue struct {
	IntValue    string `json:"intValue"`
	StringValue string `json:"stringValue"`
}

type TestTraceAttribute struct {
	Key   string                  `json:"key"`
	Value TestTraceAttributeValue `json:"value,omitempty"`
}

// TestTraceReport holds references to the File exporter defined by the otel-collector
type TestTraceReport struct {
	ResourceSpans []struct {
		Resource struct {
			Attributes []TestTraceAttribute `json:"attributes"`
		} `json:"resource"`
		InstrumentationLibrarySpans []struct {
			InstrumentationLibrary struct {
				Name string `json:"name"`
			} `json:"instrumentationLibrary"`
			Spans []struct {
				TraceID           string               `json:"traceId"`
				SpanID            string               `json:"spanId"`
				ParentSpanID      string               `json:"parentSpanId"`
				Name              string               `json:"name"`
				Kind              string               `json:"kind"`
				StartTimeUnixNano string               `json:"startTimeUnixNano"`
				EndTimeUnixNano   string               `json:"endTimeUnixNano"`
				Attributes        []TestTraceAttribute `json:"attributes"`
				Status            struct {
				} `json:"status"`
			} `json:"spans"`
		} `json:"instrumentationLibrarySpans"`
	} `json:"resourceSpans"`
}

func assertStringValueInAttribute(t *testing.T, att TestTraceAttributeValue, expected string) {
	assert.Equal(t, expected, att.StringValue)
}

func findAttributeInArray(attributes []TestTraceAttribute, key string) (TestTraceAttribute, error) {
	for _, att := range attributes {
		if att.Key == key {
			return att, nil
		}
	}

	return TestTraceAttribute{}, fmt.Errorf("attribute with key '%s' not found", key)
}

func Test_Main(t *testing.T) {
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

	err = Main(context.Background(), &TestReader{})
	if err != nil {
		t.Error()
	}

	// TODO: retry until the file is written by the otel-exporter
	time.Sleep(time.Minute)

	// assert using the generated file
	jsonBytes, _ := ioutil.ReadFile(reportFilePath)

	var tracesReport TestTraceReport
	err = json.Unmarshal(jsonBytes, &tracesReport)
	if err != nil {
		t.Error(err.Error())
	}

	resourceSpans := tracesReport.ResourceSpans[0]

	srvNameAttribute := resourceSpans.Resource.Attributes[0]
	assert.Equal(t, "service.name", srvNameAttribute.Key)
	assertStringValueInAttribute(t, srvNameAttribute.Value, "jaeger-srv-test")

	srvVersionAttribute := resourceSpans.Resource.Attributes[1]
	assert.Equal(t, "service.version", srvVersionAttribute.Key)
	assertStringValueInAttribute(t, srvVersionAttribute.Value, "")

	instrumentationLibrarySpans := resourceSpans.InstrumentationLibrarySpans[0]

	assert.Equal(t, "jaeger-srv-test", instrumentationLibrarySpans.InstrumentationLibrary.Name)

	spans := instrumentationLibrarySpans.Spans

	// there are 15 elements:
	//   1 testsuites element (root element)
	// 	 3 testsuite element
	// 	 11 testcase elements
	assert.Equal(t, 15, len(spans))

	aTestCase := spans[2]
	assert.Equal(t, "TestCheckConfigDirsCreatesWorkspaceAtHome", aTestCase.Name)
	assert.Equal(t, "SPAN_KIND_INTERNAL", aTestCase.Kind)

	codeFunction, _ := findAttributeInArray(aTestCase.Attributes, "code.function")
	assertStringValueInAttribute(t, codeFunction.Value, "TestCheckConfigDirsCreatesWorkspaceAtHome")

	testClassName, _ := findAttributeInArray(aTestCase.Attributes, "test.classname")
	assertStringValueInAttribute(t, testClassName.Value, "github.com/elastic/e2e-testing/cli/config")

	goVersion, _ := findAttributeInArray(aTestCase.Attributes, "go.version")
	assertStringValueInAttribute(t, goVersion.Value, "go1.16.3 linux/amd64")
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
