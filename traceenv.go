package main

import (
	"context"
	"os"

	"go.opentelemetry.io/otel/propagation"
)

const (
	traceparentHeader = "traceparent"
	tracestateHeader  = "tracestate"
)

// see https://github.com/moby/buildkit/pull/2572
func initOtelContext(ctx context.Context) context.Context {
	// open-telemetry/opentelemetry-specification#740
	parent := os.Getenv("TRACEPARENT")
	state := os.Getenv("TRACESTATE")

	if parent != "" {
		tc := propagation.TraceContext{}
		return tc.Extract(ctx, &textMap{parent: parent, state: state})
	}

	return ctx
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
