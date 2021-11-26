package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
)

var workingDir string
var tempDir string

func init() {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalln("Cannot get current working dir, which is needed by tests")
	}

	workingDir = wd
}

// FakeGitRepo downloads Octocat's hello-world repository from Github, providing a simple DSL to add/remove files and commit them into
// the fake git repository
type FakeGitRepo struct {
	repo             *git.Repository
	divergesFromBase bool
	repoPath         string
	t                *testing.T
}

func NewFakeGitRepo(t *testing.T) *FakeGitRepo {
	cloneURL := "https://github.com/octocat/hello-world"
	tempDir = t.TempDir()

	repo, err := git.PlainClone(tempDir, false, &git.CloneOptions{URL: cloneURL})
	if err != nil {
		fmt.Printf(">> could not initialise test repo")
		return nil
	}

	return &FakeGitRepo{repo: repo, repoPath: tempDir, t: t}
}

func (r *FakeGitRepo) withBranch(branchName string) *FakeGitRepo {
	workTree, err := r.repo.Worktree()
	if err != nil {
		r.t.Errorf(">> could not get worktree: %v", err)
		return r
	}

	err = workTree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName(branchName),
		Create: true,
		Force:  true,
	})
	if err != nil {
		r.t.Errorf(">> could not checkout to branch: %v", err)
		return r
	}

	r.divergesFromBase = true

	return r
}

func (r *FakeGitRepo) addingFile(file string) *FakeGitRepo {
	workTree, err := r.repo.Worktree()
	if err != nil {
		r.t.Errorf(">> could not retrieve worktree: %v", err)
		return r
	}

	// copy sample file
	sampleFile := path.Join(workingDir, file)
	targetFile := path.Join(tempDir, file)

	sourceFileStat, err := os.Stat(sampleFile)
	if err != nil {
		r.t.Errorf(">> could not stat sample file: %v", err)
		return r
	}

	if !sourceFileStat.Mode().IsRegular() {
		r.t.Errorf(">> sample file is not a regular file: %v", err)
		return r
	}

	source, err := os.Open(sampleFile)
	if err != nil {
		r.t.Errorf(">> could not opem sample file: %v", err)
		return r
	}
	defer source.Close()

	destination, err := os.Create(targetFile)
	if err != nil {
		r.t.Errorf(">> could not create target file: %v", err)
		return r
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		r.t.Errorf(">> could not copy file: %v", err)
		return r
	}

	_, err = workTree.Add(file)
	if err != nil {
		r.t.Errorf(">> could not git-add the file")
		return r
	}

	return r
}

func (r *FakeGitRepo) withCommit(message string) *FakeGitRepo {
	workTree, err := r.repo.Worktree()
	if err != nil {
		r.t.Errorf(">> could not retrieve worktree: %v", err)
		return r
	}

	_, err = workTree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Author Test",
			Email: "author@test.com",
			When:  time.Now(),
		},
		Committer: &object.Signature{
			Name:  "Committer Test",
			Email: "committer@test.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		r.t.Errorf(">> could not git-commit the file: %v", err)
		return nil
	}

	r.divergesFromBase = true

	return r
}

func (r *FakeGitRepo) removingFile(file string) *FakeGitRepo {
	workTree, err := r.repo.Worktree()
	if err != nil {
		r.t.Errorf(">> could not retrieve worktree: %v", err)
		return r
	}

	targetFile := path.Join(tempDir, file)
	err = os.Remove(targetFile)
	if err != nil {
		r.t.Errorf(">> could not remove target file: %v", err)
		return r
	}

	_, err = workTree.Add(file)
	if err != nil {
		r.t.Errorf(">> could not git-add the file")
		return r
	}

	return r
}

func (r *FakeGitRepo) read() *GitScm {
	scm := NewGitScm(r.repoPath)

	currentBranch, err := r.repo.Head()
	if err != nil {
		r.t.Errorf(">> could not get head branch for fake repo: %v", err)
	}

	master, err := r.repo.Branch("master")
	if err != nil {
		r.t.Errorf(">> could not get master branch for fake repo: %v", err)
	}

	scm.baseRef = master.Name
	scm.headSha = currentBranch.Hash().String()
	scm.changeRequest = r.divergesFromBase

	return scm
}

func TestCheckGitProvider(t *testing.T) {
	t.Run("Github", func(t *testing.T) {
		originalSha := os.Getenv("GITHUB_SHA")
		originalBaseRef := os.Getenv("GITHUB_BASE_REF")
		originalHeadRef := os.Getenv("GITHUB_HEAD_REF")

		testSha := "0123456"
		if originalSha != "" {
			testSha = originalSha
		}

		testBaseRef := "main"
		if originalBaseRef != "" {
			testBaseRef = originalBaseRef
		}

		testHeadRef := "feature/pr-23"
		if originalHeadRef != "" {
			testHeadRef = originalHeadRef
		}

		cleanUpFn := func() {
			os.Setenv("GITHUB_SHA", originalSha)
			os.Setenv("GITHUB_BASE_REF", originalBaseRef)
			os.Setenv("GITHUB_HEAD_REF", originalHeadRef)
		}

		t.Run("Running for Branches", func(t *testing.T) {
			os.Setenv("GITHUB_SHA", testSha)
			defer cleanUpFn()

			sha, baseRef, provider, changeRequest := checkGitProvider()
			assert.Equal(t, testSha, sha)
			assert.Equal(t, testBaseRef, baseRef)
			assert.Equal(t, "Github", provider)
			assert.False(t, changeRequest)
		})

		t.Run("Running for Pull Requests", func(t *testing.T) {
			os.Setenv("GITHUB_SHA", testSha)
			os.Setenv("GITHUB_BASE_REF", testBaseRef)
			os.Setenv("GITHUB_HEAD_REF", testHeadRef)
			defer cleanUpFn()

			sha, baseRef, provider, changeRequest := checkGitProvider()
			assert.Equal(t, testSha, sha)
			assert.Equal(t, testBaseRef, baseRef)
			assert.Equal(t, "Github", provider)
			assert.True(t, changeRequest)
		})
	})

	t.Run("Gitlab", func(t *testing.T) {
		originalCommitBranch := os.Getenv("CI_COMMIT_BRANCH")
		originalSourceBranchSha := os.Getenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA")
		originalTargetBranchName := os.Getenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME")

		cleanUpFn := func() {
			os.Setenv("CI_COMMIT_BRANCH", originalCommitBranch)
			os.Setenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA", originalSourceBranchSha)
			os.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", originalTargetBranchName)
		}

		t.Run("Running for Branches", func(t *testing.T) {
			os.Setenv("CI_COMMIT_BRANCH", "branch")
			os.Setenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA", "0123456")
			os.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "main")
			defer cleanUpFn()

			sha, baseRef, provider, changeRequest := checkGitProvider()
			assert.Equal(t, "0123456", sha)
			assert.Equal(t, "main", baseRef)
			assert.Equal(t, "Gitlab", provider)
			assert.False(t, changeRequest)
		})

		t.Run("Running for Merge Requests", func(t *testing.T) {
			os.Setenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA", "0123456")
			os.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "main")
			defer cleanUpFn()

			sha, baseRef, provider, changeRequest := checkGitProvider()
			assert.Equal(t, "0123456", sha)
			assert.Equal(t, "main", baseRef)
			assert.Equal(t, "Gitlab", provider)
			assert.True(t, changeRequest)
		})
	})

	t.Run("Local machine", func(t *testing.T) {
		t.Run("Running with TARGET_BRANCH", func(t *testing.T) {
			os.Setenv("TARGET_BRANCH", "main")
			defer os.Unsetenv("TARGET_BRANCH")
			sha, baseRef, provider, changeRequest := checkGitProvider()
			assert.Equal(t, "", sha)
			assert.Equal(t, "main", baseRef)
			assert.Equal(t, "", provider)
			assert.False(t, changeRequest)
		})

		t.Run("Running without TARGET_BRANCH", func(t *testing.T) {
			sha, baseRef, provider, changeRequest := checkGitProvider()
			assert.Equal(t, "", sha)
			assert.Equal(t, "", baseRef)
			assert.Equal(t, "", provider)
			assert.False(t, changeRequest)
		})
	})
}

func TestGit_ContributeAttributesForChangeRequests(t *testing.T) {
	os.Setenv("TARGET_BRANCH", "master") // master branch is the base branch for the fake repository (octocat/hello-world)
	defer os.Unsetenv("TARGET_BRANCH")

	scm := NewFakeGitRepo(t).withBranch("this-is-a-test-branch").addingFile("TEST-sample2.xml").removingFile("README").withCommit("This is a test commit").read()
	if scm == nil {
		t.FailNow()
	}

	atts := scm.contributeAttributes()

	assert.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmAuthors, "author@test.com") }, "Authors should be set as scm.authors. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmCommitters, "committer@test.com") }, "Committers should be set as scm.committers. Attributes: %v", atts)

	if scm.changeRequest {
		// we are adding 1 file with 202 lines, and we are deleting 1 file with 1 line
		assert.Condition(t, func() bool { return keyExistsWithIntValue(t, atts, GitAdditions, 202) }, "Additions should be set as scm.git.additions. Attributes: %v", atts)
		assert.Condition(t, func() bool { return keyExistsWithIntValue(t, atts, GitDeletions, 1) }, "Deletions should be set as scm.git.deletions. Attributes: %v", atts)
		assert.Condition(t, func() bool { return keyExistsWithIntValue(t, atts, GitModifiedFiles, 2) }, "Modified files should be set as scm.git.modified.files. Attributes: %v", atts)
	}

	assert.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmBranch, "HEAD") }, "should be set as scm.branch. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmType, "git") }, "Git should be set as scm.type. Attributes: %v", atts)
	assert.Condition(t, func() bool {
		return keyExistsWithValue(t, atts, ScmRepository, "https://github.com/octocat/hello-world")
	}, "Remote should be set as scm.repository. Attributes: %v", atts)
}

func TestGit_ContributeAttributesForBranches(t *testing.T) {
	scm := NewFakeGitRepo(t).read()
	if scm == nil {
		t.FailNow()
	}

	atts := scm.contributeAttributes()

	assert.Condition(t, func() bool { return !keyExists(t, atts, ScmAuthors) }, "Authors shouldn't be set as scm.authors. Attributes: %v", atts)
	assert.Condition(t, func() bool { return !keyExists(t, atts, ScmCommitters) }, "Committers shouldn't be set as scm.committers. Attributes: %v", atts)

	if scm.changeRequest {
		assert.Condition(t, func() bool { return !keyExists(t, atts, GitAdditions) }, "Additions shouldn't be as scm.git.additions. Attributes: %v", atts)
		assert.Condition(t, func() bool { return !keyExists(t, atts, GitDeletions) }, "Deletions shouldn't be as scm.git.deletions. Attributes: %v", atts)
		assert.Condition(t, func() bool { return !keyExists(t, atts, GitModifiedFiles) }, "Modified files shouldn't be as scm.git.modified.files. Attributes: %v", atts)
	}

	assert.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmBranch, "refs/heads/master") }, "Branch should be set as scm.branch. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmType, "git") }, "Git should be set as scm.type. Attributes: %v", atts)
	assert.Condition(t, func() bool {
		return keyExistsWithValue(t, atts, ScmRepository, "https://github.com/octocat/hello-world")
	}, "Remote should be set as scm.repository. Attributes: %v", atts)
}

func TestGit_ContributeCommitters(t *testing.T) {
	os.Setenv("TARGET_BRANCH", "master")
	defer os.Unsetenv("TARGET_BRANCH")

	scm := NewFakeGitRepo(t).withBranch("this-is-a-test-branch").addingFile("TEST-sample2.xml").withCommit("This is a test commit").read()
	if scm == nil {
		t.FailNow()
	}

	headCommit, targetCommit, err := scm.calculateCommits()
	if err != nil {
		t.Error()
	}

	atts, err := scm.contributeCommitters(headCommit, targetCommit)
	if err != nil {
		t.Error()
	}

	assert.Equal(t, 2, len(atts))
	// we are adding 1 file with 202 lines, and we are deleting 1 file with 1 line
	assert.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmAuthors, "author@test.com") }, "Authors should be set as scm.authors. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmCommitters, "committer@test.com") }, "Committers should be set as scm.committers. Attributes: %v", atts)
}

func TestGit_ContributeFilesAndLines(t *testing.T) {
	os.Setenv("TARGET_BRANCH", "master")
	defer os.Unsetenv("TARGET_BRANCH")

	scm := NewFakeGitRepo(t).withBranch("this-is-a-test-branch").addingFile("TEST-sample2.xml").removingFile("README").withCommit("This is a test commit").read()
	if scm == nil {
		t.FailNow()
	}

	headCommit, targetCommit, err := scm.calculateCommits()
	if err != nil {
		t.Error()
	}

	// TODO: verify attributes in a consistent manner on the CI. Until then, check there are no errors
	atts, err := scm.contributeFilesAndLines(headCommit, targetCommit)
	if err != nil {
		t.Error()
	}

	assert.Equal(t, 3, len(atts))
	// we are adding 1 file with 202 lines, and we are deleting 1 file with 1 line
	assert.Condition(t, func() bool { return keyExistsWithIntValue(t, atts, GitAdditions, 202) }, "Additions should be set as scm.git.additions. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExistsWithIntValue(t, atts, GitDeletions, 1) }, "Deletions should be set as scm.git.deletions. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExistsWithIntValue(t, atts, GitModifiedFiles, 2) }, "Modified files should be set as scm.git.modified.files. Attributes: %v", atts)
}

func TestGit_CalculateCommitsForChangeRequests(t *testing.T) {
	os.Setenv("TARGET_BRANCH", "master")
	defer os.Unsetenv("TARGET_BRANCH")

	scm := NewFakeGitRepo(t).withBranch("this-is-a-test-branch").addingFile("TEST-sample2.xml").removingFile("README").withCommit("This is a test commit").read()
	if scm == nil {
		t.FailNow()
	}

	headCommit, targetCommit, err := scm.calculateCommits()
	if err != nil {
		t.Error()
	}

	assert.NotNil(t, headCommit)
	assert.NotNil(t, targetCommit)
}

func TestGit_CalculateCommitsForBranches(t *testing.T) {
	scm := NewFakeGitRepo(t).read()
	if scm == nil {
		t.FailNow()
	}

	headCommit, targetCommit, err := scm.calculateCommits()
	if err != nil {
		t.Error()
	}

	assert.Equal(t, headCommit, targetCommit)
}

func keyExists(t *testing.T, attributes []attribute.KeyValue, key string) bool {
	for _, att := range attributes {
		if string(att.Key) == key {
			return true
		}
	}

	return false
}

func keyExistsWithIntValue(t *testing.T, attributes []attribute.KeyValue, key string, value ...int64) bool {
	for _, att := range attributes {
		if string(att.Key) == key {
			for _, v := range value {
				val := att.Value.AsInt64Slice()
				if len(val) == 0 {
					return v == att.Value.AsInt64()
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
