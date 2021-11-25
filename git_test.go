package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
)

func TestGit_ContributeAttributes_WithCommitters(t *testing.T) {
	os.Setenv("TARGET_BRANCH", "main")
	defer os.Unsetenv("TARGET_BRANCH")

	workingDir, err := os.Getwd()
	if err != nil {
		t.Error()
	}

	scm := &GitScm{
		repositoryPath: workingDir,
	}

	atts := scm.contributeAttributes()

	assert.Condition(t, func() bool { return keyExists(t, atts, GitAdditions) }, "Additions is not set as scm.git.additions. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExists(t, atts, GitDeletions) }, "Deletions is not set as scm.git.deletions. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExists(t, atts, GitModifiedFiles) }, "Modified files is not set as scm.git.modified.files. Attributes: %v", atts)

	assert.Condition(t, func() bool { return keyExists(t, atts, ScmAuthors) }, "Authors is not set as scm.authors. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExists(t, atts, ScmBranch) }, "Branch is not set as scm.branch. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExists(t, atts, ScmCommitters) }, "Committers is not set as scm.committers. Attributes: %v", atts)
	assert.Condition(t, func() bool {
		return keyExistsWithValue(t, atts, ScmType, "git")
	}, "Git is not set as scm.type. Attributes: %v", atts)
	assert.Condition(t, func() bool {
		// check that any of the git or https protocols are set as scm.repository
		return keyExistsWithValue(t, atts, ScmRepository, "git@github.com:mdelapenya/junit2otlp.git", "https://github.com/mdelapenya/junit2otlp")
	}, "Remote is not set as scm.repository. Attributes: %v", atts)
}

func TestGit_ContributeAttributes_WithoutCommitters(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Error()
	}

	scm := &GitScm{
		repositoryPath: workingDir,
	}

	atts := scm.contributeAttributes()

	assert.Condition(t, func() bool { return !keyExists(t, atts, ScmAuthors) }, "Authors is not set as scm.authors. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExists(t, atts, ScmBranch) }, "Branch is not set as scm.branch. Attributes: %v", atts)
	assert.Condition(t, func() bool { return !keyExists(t, atts, ScmCommitters) }, "Committers is not set as scm.committers. Attributes: %v", atts)
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

	headCommit, targetCommit, err := calculateCommits(repository)
	if err != nil {
		t.Error()
	}

	// TODO: verify attributes in a consistent manner on the CI. UNtil then, check there are no errors
	_, err = contributeCommitters(repository, headCommit, targetCommit)
	if err != nil {
		t.Error()
	}
}

func TestGit_CalculateCommitsWithTargetBranch(t *testing.T) {
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

	headCommit, targetCommit, err := calculateCommits(repository)
	if err != nil {
		t.Error()
	}

	assert.NotNil(t, headCommit)
	assert.NotNil(t, targetCommit)
}

func TestGit_CalculateCommitsWithoutTargetBranch(t *testing.T) {
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

	headCommit, targetCommit, err := calculateCommits(repository)
	if err == nil {
		t.Error()
	}

	assert.Nil(t, headCommit)
	assert.Nil(t, targetCommit)
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
