package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"go.opentelemetry.io/otel/attribute"
)

const propertiesAllowAll = "all"
const Junit2otlp = "junit2otlp"

type Config struct {
	// Path to the SCM repository to be read
	RepositoryPath string

	// OpenTelemetry Service Name to be used when sending traces and metrics for the jUnit report
	ServiceName string
	// OpenTelemetry Service Version to be used when sending traces and metrics for the jUnit report
	ServiceVersion string
	// OpenTelemetry Trace Name to be used when sending traces and metrics for the jUnit report
	TraceName string
	// List of attributes to be added to the jUnit report
	AdditionalAttributes []attribute.KeyValue

	// Properties to be allowed in the jUnit report
	allPropertiesAllowed bool
	propertiesAllowed    []string

	// Maximum export batch size allowed when creating a BatchSpanProcessor.
	// Default is 10
	BatchSize int
	// Skip sending traces to the OpenTelemetry collector
	SkipTraces bool
	// Skip sending metrics to the OpenTelemetry collector
	SkipMetrics bool
}

func NewConfigFromDefaults() *Config {
	return &Config{
		RepositoryPath: GetDefaultwd(),

		ServiceName:          "",
		ServiceVersion:       "",
		TraceName:            Junit2otlp,
		AdditionalAttributes: nil,

		allPropertiesAllowed: true,
		propertiesAllowed:    []string{},

		BatchSize:   10,
		SkipTraces:  false,
		SkipMetrics: false,
	}
}

func NewConfigFromArgs() (*Config, error) {
	const defaultMaxBatchSize = 10

	var batchSizeFlag int
	var repositoryPathFlag string
	var serviceNameFlag string
	var serviceVersionFlag string
	var traceNameFlag string
	var propertiesAllowedString string
	var additionalAttributes string
	var skipTracesFlag bool
	var skipMetricsFlag bool

	flag.IntVar(&batchSizeFlag, "batch-size", defaultMaxBatchSize, "Maximum export batch size allowed when creating a BatchSpanProcessor")
	flag.StringVar(&repositoryPathFlag, "repository-path", GetDefaultwd(), "Path to the SCM repository to be read")
	flag.StringVar(&serviceNameFlag, "service-name", "", "OpenTelemetry Service Name to be used when sending traces and metrics for the jUnit report")
	flag.StringVar(&serviceVersionFlag, "service-version", "", "OpenTelemetry Service Version to be used when sending traces and metrics for the jUnit report")
	flag.StringVar(&traceNameFlag, "trace-name", Junit2otlp, "OpenTelemetry Trace Name to be used when sending traces and metrics for the jUnit report")
	flag.StringVar(&propertiesAllowedString, "properties-allowed", propertiesAllowAll, "Comma separated list of properties to be allowed in the jUnit report")
	flag.StringVar(&additionalAttributes, "additional-attributes", "", "Comma separated list of attributes to be added to the jUnit report")
	flag.BoolVar(&skipTracesFlag, "traces-skip-sending", false, "Skip sending traces to the OpenTelemetry collector")
	flag.BoolVar(&skipMetricsFlag, "metrics-skip-sending", false, "Skip sending metrics to the OpenTelemetry collector")
	flag.Parse()

	additionalAttrs, err := processAdditionalAttributes(additionalAttributes)
	if err != nil {
		return nil, err
	}

	return &Config{
		RepositoryPath: repositoryPathFlag,

		ServiceName:          getOtlpServiceName(serviceNameFlag),
		ServiceVersion:       getOtlpServiceVersion(serviceVersionFlag),
		TraceName:            traceNameFlag,
		AdditionalAttributes: additionalAttrs,

		allPropertiesAllowed: propertiesAllowedString == propertiesAllowAll,
		propertiesAllowed:    propertiesAllowed(propertiesAllowedString),

		BatchSize:   batchSizeFlag,
		SkipTraces:  skipTracesFlag,
		SkipMetrics: skipMetricsFlag,
	}, nil
}

// IsPropertyAllowed checks if a property is allowed in the jUnit report
func (c *Config) IsPropertyAllowed(property string) bool {
	// if propertiesAllowedString is not "all" (default) and the key is not in the
	// allowed list, skip it
	if c.allPropertiesAllowed {
		return true
	}
	return slices.Contains(c.propertiesAllowed, property)
}

// GetDefaultwd retrieves the current working dir, using '.' in the case an error occurs
func GetDefaultwd() string {
	workingDir, err := os.Getwd()
	if err != nil {
		return "."
	}

	return workingDir
}

func propertiesAllowed(allowedString string) []string {
	propsAllowed := []string{}

	if allowedString == "" {
		return propsAllowed
	}

	allowed := strings.Split(allowedString, ",")
	for _, prop := range allowed {
		propsAllowed = append(propsAllowed, strings.TrimSpace(prop))
	}

	return propsAllowed
}

// getOtlpEnvVar the precedence order is: flag > env var > fallback
func getOtlpEnvVar(flag string, envVarKey string, fallback string) string {
	if flag != "" {
		return flag
	}

	envVar := os.Getenv(envVarKey)
	if envVar != "" {
		return envVar
	}

	return fallback
}

// getOtlpServiceName checks the service name
func getOtlpServiceName(serviceNameFlag string) string {
	return getOtlpEnvVar(serviceNameFlag, "OTEL_SERVICE_NAME", Junit2otlp)
}

// getOtlpServiceVersion checks the service version
func getOtlpServiceVersion(serviceVersionFlag string) string {
	return getOtlpEnvVar(serviceVersionFlag, "OTEL_SERVICE_VERSION", "")
}

func processAdditionalAttributes(additionalAttributes string) ([]attribute.KeyValue, error) {
	additionalAttrs := []attribute.KeyValue{}

	// add additional attributes if provided to the runtime attributes
	if additionalAttributes != "" {
		additionalAttrsErrors := []error{}

		addAttrs := strings.Split(additionalAttributes, ",")
		for _, attr := range addAttrs {
			kv := strings.Split(attr, "=")
			if len(kv) == 2 {
				additionalAttrs = append(additionalAttrs, attribute.Key(kv[0]).String(kv[1]))
			} else {
				additionalAttrsErrors = append(additionalAttrsErrors,
					fmt.Errorf("invalid attribute: %s", attr))
			}
		}

		if err := errors.Join(additionalAttrsErrors...); err != nil {
			return nil, fmt.Errorf("failed to add additional attributes: %w", err)
		}
	}

	return additionalAttrs, nil
}
