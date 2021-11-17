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

func Test_GetServiceName(t *testing.T) {
	t.Run("Read from environment", func(t *testing.T) {
		t.Run("Without environment variable retrieves fallback", func(t *testing.T) {
			os.Unsetenv("OTEL_SERVICE_NAME")
			defer resetEnvironment()

			srvName := getOtlpServiceName()

			assert.Equal(t, Junit2otlp, srvName)
		})

		t.Run("With environment variable retrieves the variable", func(t *testing.T) {
			os.Setenv("OTEL_SERVICE_NAME", "foobar")
			defer resetEnvironment()

			srvName := getOtlpServiceName()

			assert.Equal(t, "foobar", srvName)
		})
	})

	t.Run("Read from command line flag", func(t *testing.T) {
		t.Run("Without environment variable retrieves the flag", func(t *testing.T) {
			os.Unsetenv("OTEL_SERVICE_NAME")
			serviceNameFlag = "this-is-a-flag"
			defer resetEnvironment()

			srvName := getOtlpServiceName()

			assert.Equal(t, "this-is-a-flag", srvName)
			assert.Equal(t, "this-is-a-flag", os.Getenv("OTEL_SERVICE_NAME"))
		})

		t.Run("With environment variable retrieves the variable", func(t *testing.T) {
			os.Setenv("OTEL_SERVICE_NAME", "foobar")
			serviceNameFlag = "this-is-a-flag"
			defer resetEnvironment()

			srvName := getOtlpServiceName()

			assert.Equal(t, "foobar", srvName)
			assert.Equal(t, "foobar", os.Getenv("OTEL_SERVICE_NAME"))
		})
	})
}

func resetEnvironment() {
	os.Setenv("OTEL_SERVICE_NAME", originalEnvVar)

	serviceNameFlag = originalServiceNameFlag
}
