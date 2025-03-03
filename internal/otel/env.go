package otel

import (
	"context"
	"os"
	"runtime"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

const (
	traceparentHeader = "traceparent"
	tracestateHeader  = "tracestate"
)

// see https://github.com/moby/buildkit/pull/2572
func InitOtelContext(ctx context.Context) context.Context {
	// open-telemetry/opentelemetry-specification#740
	parent := os.Getenv("TRACEPARENT")
	state := os.Getenv("TRACESTATE")

	if parent != "" {
		tc := propagation.TraceContext{}
		return tc.Extract(ctx, &textMap{parent: parent, state: state})
	}

	return ctx
}

func RuntimeAttributes() []attribute.KeyValue {
	return []attribute.KeyValue{
		semconv.HostArchKey.String(runtime.GOARCH),
		semconv.OSNameKey.String(runtime.GOOS),
	}
}

type textMap struct {
	parent string
	state  string
}

func (tm *textMap) Get(key string) string {
	switch key {
	case traceparentHeader:
		return tm.parent
	case tracestateHeader:
		return tm.state
	default:
		return ""
	}
}

func (tm *textMap) Set(key string, value string) {
}

func (tm *textMap) Keys() []string {
	return []string{traceparentHeader, tracestateHeader}
}
