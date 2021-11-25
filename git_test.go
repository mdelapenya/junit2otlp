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

	atts := scm.contributeAttributes()

	assert.Equal(t, 3, len(atts))

	assert.Condition(t, func() bool { return keyExists(t, atts, ScmBranch) }, "Branch is not set as scm.branch. Attributes: %v", atts)
	assert.Condition(t, func() bool {
		return keyExistsWithValue(t, atts, ScmType, "git")
	}, "Git is not set as scm.type. Attributes: %v", atts)
	assert.Condition(t, func() bool {
		// check that any of the git or https protocols are set as scm.repository
		return keyExistsWithValue(t, atts, ScmRepository, "git@github.com:mdelapenya/junit2otlp.git", "https://github.com/mdelapenya/junit2otlp")
	}, "Remote is not set as scm.repository. Attributes: %v", atts)
}

func TestGit_ContributeCommitters(t *testing.T) {
	os.Setenv("TARGET_BRANCH", "main")
	defer os.Unsetenv("TARGET_BRANCH")

	workingDir, err := os.Getwd()
	if err != nil {
		t.Error()
	}

	scm := &GitScm{
		repositoryPath: workingDir,
	}

	repository, err := scm.openLocalRepository()
	if err != nil {
		t.Error()
	}

	// TODO: verify attributes in a consistent manner on the CI. UNtil then, check there are no errors
	_, err = contributeCommitters(repository)
	if err != nil {
		t.Error()
	}
}

func TestGit_ContributeCommittersWithoutTargetBranch(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Error()
	}

	scm := &GitScm{
		repositoryPath: workingDir,
	}

	repository, err := scm.openLocalRepository()
	if err != nil {
		t.Error()
	}

	atts, err := contributeCommitters(repository)
	assert.NotNil(t, err)
	assert.Empty(t, atts)
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
				val := att.Value.AsStringSlice()
				if len(val) == 0 {
					return v == att.Value.AsString()
				}

				for _, vv := range val {
					if vv == v {
						return true
					}
				}
			}
		}
	}

	return false
}
