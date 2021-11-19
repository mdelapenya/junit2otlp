package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
)

func TestGit(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Error()
	}

	scm := &GitScm{
		repositoryPath: workingDir,
	}

	atts := scm.contributeOtelAttributes()

	assert.Equal(t, 3, len(atts))

	assert.Condition(t, func() bool { return keyExists(t, atts, ScmBranch) }, "Branch is not set as scm.branch")
	assert.Condition(t, func() bool {
		return keyExistsWithValue(t, atts, ScmType, "git")
	}, "Git is not set as scm.type")
	assert.Condition(t, func() bool {
		return keyExistsWithValue(t, atts, ScmRepository, "git@github.com:mdelapenya/junit2otlp.git")
	}, "Remote is not set as scm.repository")
}

func keyExists(t *testing.T, attributes []attribute.KeyValue, key string) bool {
	for _, att := range attributes {
		if string(att.Key) == key {
			return true
		}
	}

	return false
}

func keyExistsWithValue(t *testing.T, attributes []attribute.KeyValue, key string, value string) bool {
	for _, att := range attributes {
		if string(att.Key) == key && att.Value.AsString() == value {
			return true
		}
	}

	return false
}
