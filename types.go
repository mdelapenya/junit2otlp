package main

import (
	"go.opentelemetry.io/otel/attribute"
)

type OTELAttributesContributor interface {
	contributeAttributes() []attribute.KeyValue
}
