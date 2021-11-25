package main

import (
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
)

var workingDir string

func init() {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalln("Cannot get current working dir, which is needed by tests")
	}

	workingDir = wd
}

func TestCheckGitProvider(t *testing.T) {
	t.Run("Running on Github for Branches", func(t *testing.T) {
		os.Setenv("GITHUB_SHA", "0123456")
		defer func() {
			os.Unsetenv("GITHUB_SHA")
		}()

		sha, baseRef, provider, request := checkGitProvider()
		assert.Equal(t, "0123456", sha)
		assert.Equal(t, "", baseRef)
		assert.Equal(t, "Github", provider)
		assert.False(t, request)
	})

	t.Run("Running on Github for Pull Requests", func(t *testing.T) {
		os.Setenv("GITHUB_SHA", "0123456")
		os.Setenv("GITHUB_BASE_REF", "main")
		os.Setenv("GITHUB_HEAD_REF", "feature/pr-23")
		defer func() {
			os.Unsetenv("GITHUB_SHA")
			os.Unsetenv("GITHUB_BASE_REF")
			os.Unsetenv("GITHUB_HEAD_REF")
		}()

		sha, baseRef, provider, request := checkGitProvider()
		assert.Equal(t, "0123456", sha)
		assert.Equal(t, "main", baseRef)
		assert.Equal(t, "Github", provider)
		assert.True(t, request)
	})

	t.Run("Running on Gitlab for Branches", func(t *testing.T) {
		os.Setenv("CI_COMMIT_BRANCH", "branch")
		os.Setenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA", "0123456")
		os.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "main")
		defer func() {
			os.Unsetenv("CI_COMMIT_BRANCH")
			os.Unsetenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA")
			os.Unsetenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME")
		}()

		sha, baseRef, provider, request := checkGitProvider()
		assert.Equal(t, "0123456", sha)
		assert.Equal(t, "main", baseRef)
		assert.Equal(t, "Gitlab", provider)
		assert.False(t, request)
	})

	t.Run("Running on Gitlab for Merge Requests", func(t *testing.T) {
		os.Setenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA", "0123456")
		os.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "main")
		defer func() {
			os.Unsetenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA")
			os.Unsetenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME")
		}()

		sha, baseRef, provider, request := checkGitProvider()
		assert.Equal(t, "0123456", sha)
		assert.Equal(t, "main", baseRef)
		assert.Equal(t, "Gitlab", provider)
		assert.True(t, request)
	})

	t.Run("Running on Local machine with TARGET_BRANCH", func(t *testing.T) {
		os.Setenv("TARGET_BRANCH", "main")
		defer os.Unsetenv("TARGET_BRANCH")
		sha, baseRef, provider, request := checkGitProvider()
		assert.Equal(t, "", sha)
		assert.Equal(t, "main", baseRef)
		assert.Equal(t, "", provider)
		assert.False(t, request)
	})

	t.Run("Running on Local machine without TARGET_BRANCH", func(t *testing.T) {
		sha, baseRef, provider, request := checkGitProvider()
		assert.Equal(t, "", sha)
		assert.Equal(t, "", baseRef)
		assert.Equal(t, "", provider)
		assert.False(t, request)
	})
}

func TestGit_ContributeAttributes_WithCommitters(t *testing.T) {
	os.Setenv("TARGET_BRANCH", "main")
	defer os.Unsetenv("TARGET_BRANCH")

	scm := NewGitScm(workingDir)

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
	scm := NewGitScm(workingDir)

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

	scm := NewGitScm(workingDir)

	headCommit, targetCommit, err := scm.calculateCommits()
	if err != nil {
		t.Error()
	}

	// TODO: verify attributes in a consistent manner on the CI. UNtil then, check there are no errors
	_, err = scm.contributeCommitters(headCommit, targetCommit)
	if err != nil {
		t.Error()
	}
}

func TestGit_ContributeFilesAndLines(t *testing.T) {
	os.Setenv("TARGET_BRANCH", "main")
	defer os.Unsetenv("TARGET_BRANCH")

	scm := NewGitScm(workingDir)

	headCommit, targetCommit, err := scm.calculateCommits()
	if err != nil {
		t.Error()
	}

	// TODO: verify attributes in a consistent manner on the CI. Until then, check there are no errors
	_, err = scm.contributeFilesAndLines(headCommit, targetCommit)
	if err != nil {
		t.Error()
	}
}

func TestGit_CalculateCommitsWithTargetBranch(t *testing.T) {
	os.Setenv("TARGET_BRANCH", "main")
	defer os.Unsetenv("TARGET_BRANCH")

	scm := NewGitScm(workingDir)

	headCommit, targetCommit, err := scm.calculateCommits()
	if err != nil {
		t.Error()
	}

	assert.NotNil(t, headCommit)
	assert.NotNil(t, targetCommit)
}

func TestGit_CalculateCommitsWithoutTargetBranch(t *testing.T) {
	scm := NewGitScm(workingDir)

	headCommit, targetCommit, err := scm.calculateCommits()
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
