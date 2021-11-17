package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var originalEnvVar string
var originalServiceNameFlag string

func init() {
	originalEnvVar = os.Getenv("OTEL_SERVICE_NAME")
	originalServiceNameFlag = serviceNameFlag
}

func Test_GetServiceVariable(t *testing.T) {
	var otlpTests = []struct {
		fallback       string
		getFn          func() string
		setFlag        func()
		variableSuffix string
	}{
		{
			fallback: Junit2otlp,
			getFn:    getOtlpServiceName,
			setFlag: func() {
				serviceNameFlag = "this-is-a-flag"
			},
			variableSuffix: "SERVICE_NAME",
		},
		{
			fallback: "",
			getFn:    getOtlpServiceVersion,
			setFlag: func() {
				serviceVersionFlag = "this-is-a-flag"
			},
			variableSuffix: "SERVICE_VERSION",
		},
	}

	for _, otlpotlpTest := range otlpTests {
		t.Run("Read "+otlpotlpTest.variableSuffix+" from environment", func(t *testing.T) {
			t.Run("Without environment variable retrieves fallback", func(t *testing.T) {
				os.Unsetenv("OTEL_" + otlpotlpTest.variableSuffix)
				defer resetEnvironment(otlpotlpTest.variableSuffix)

				actualValue := otlpotlpTest.getFn()

				assert.Equal(t, otlpotlpTest.fallback, actualValue)
			})

			t.Run("With environment variable retrieves the variable", func(t *testing.T) {
				os.Setenv("OTEL_"+otlpotlpTest.variableSuffix, "foobar")
				defer resetEnvironment(otlpotlpTest.variableSuffix)

				actualValue := otlpotlpTest.getFn()

				assert.Equal(t, "foobar", actualValue)
			})
		})

		t.Run("Read "+otlpotlpTest.variableSuffix+" from command line flag", func(t *testing.T) {
			t.Run("Without environment variable retrieves the flag", func(t *testing.T) {
				os.Unsetenv("OTEL_" + otlpotlpTest.variableSuffix)
				otlpotlpTest.setFlag()
				defer resetEnvironment(otlpotlpTest.variableSuffix)

				actualValue := otlpotlpTest.getFn()

				assert.Equal(t, "this-is-a-flag", actualValue)
			})

			t.Run("With environment variable retrieves the variable", func(t *testing.T) {
				os.Setenv("OTEL_"+otlpotlpTest.variableSuffix, "foobar")
				otlpotlpTest.setFlag()
				defer resetEnvironment(otlpotlpTest.variableSuffix)

				actualValue := otlpotlpTest.getFn()

				assert.Equal(t, "foobar", actualValue)
				assert.Equal(t, "foobar", os.Getenv("OTEL_"+otlpotlpTest.variableSuffix))
			})
		})
	}
}

func resetEnvironment(variableSuffix string) {
	os.Setenv("OTEL_"+variableSuffix, originalEnvVar)

	serviceNameFlag = originalServiceNameFlag
}
