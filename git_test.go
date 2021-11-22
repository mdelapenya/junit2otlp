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

	assert.Condition(t, func() bool { return keyExists(t, atts, ScmBranch) }, "Branch is not set as scm.branch. Attributes: %v", atts)
	assert.Condition(t, func() bool {
		return keyExistsWithValue(t, atts, ScmType, "git")
	}, "Git is not set as scm.type. Attributes: %v", atts)
	assert.Condition(t, func() bool {
		// check that any of the git or https protocols are set as scm.repository
		return keyExistsWithValue(t, atts, ScmRepository, "git@github.com:mdelapenya/junit2otlp.git", "https://github.com:mdelapenya/junit2otlp")
	}, "Remote is not set as scm.repository. Attributes: %v", atts)
}

func keyExists(t *testing.T, attributes []attribute.KeyValue, key string) bool {
	for _, att := range attributes {
		if string(att.Key) == key {
			return true
		}
	}

	return false
}

func keyExistsWithValue(t *testing.T, attributes []attribute.KeyValue, key string, value ...string) bool {
	for _, att := range attributes {
		if string(att.Key) == key {
			for _, v := range value {
				if att.Value.AsString() == v {
					return true
				}
			}
		}
	}

	return false
}
