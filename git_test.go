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

type CloneOptionsRequest struct {
	Depth int
	URL   string
}

func WithCloneOptions(req CloneOptionsRequest) *git.CloneOptions {
	if req.URL == "" {
		req.URL = "https://github.com/octocat/hello-world"
	}

	return &git.CloneOptions{URL: req.URL, Depth: req.Depth}
}

func NewFakeGitRepo(t *testing.T, opts *git.CloneOptions) *FakeGitRepo {
	tempDir = t.TempDir()

	repo, err := git.PlainClone(tempDir, false, opts)
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

func TestCheckGitContext(t *testing.T) {
	originalGithubSha := os.Getenv("GITHUB_SHA")
	originalGitlabBranch := os.Getenv("CI_COMMIT_BRANCH")

	t.Run("Github", func(t *testing.T) {
		// Prepare Github
		originalBaseRef := os.Getenv("GITHUB_BASE_REF")
		originalHeadRef := os.Getenv("GITHUB_HEAD_REF")
		restoreGithubFn := func() {
			os.Setenv("GITHUB_SHA", originalGithubSha)
			os.Setenv("GITHUB_BASE_REF", originalBaseRef)
			os.Setenv("GITHUB_HEAD_REF", originalHeadRef)
		}

		if originalGitlabBranch != "" {
			t.Skip("Tests skipped when running on Gitlab")
		}

		testSha := "0123456"
		if originalGithubSha != "" {
			testSha = originalGithubSha
		}

		testBaseRef := ""
		if originalBaseRef != "" {
			testBaseRef = originalBaseRef
		}

		testHeadRef := "feature/pr-23"
		if originalHeadRef != "" {
			testHeadRef = originalHeadRef
		}

		t.Run("Running for Branches", func(t *testing.T) {
			os.Setenv("GITHUB_SHA", testSha)
			defer restoreGithubFn()

			gitCtx := checkGiContext()
			assert.Equal(t, testSha, gitCtx.Commit)
			assert.Equal(t, testBaseRef, gitCtx.Branch)
			assert.Equal(t, testBaseRef, gitCtx.GetTargetBranch())
			assert.Equal(t, "Github", gitCtx.Provider)
			assert.False(t, gitCtx.ChangeRequest)
		})

		t.Run("Running for Pull Requests", func(t *testing.T) {
			os.Setenv("GITHUB_SHA", testSha)
			os.Setenv("GITHUB_REF_NAME", testHeadRef)
			os.Setenv("GITHUB_BASE_REF", "main")
			os.Setenv("GITHUB_HEAD_REF", testHeadRef)
			defer restoreGithubFn()

			gitCtx := checkGiContext()
			assert.Equal(t, testSha, gitCtx.Commit)
			assert.Equal(t, testHeadRef, gitCtx.Branch)
			assert.Equal(t, "main", gitCtx.GetTargetBranch())
			assert.Equal(t, "Github", gitCtx.Provider)
			assert.True(t, gitCtx.ChangeRequest)
		})
	})

	t.Run("Jenkins", func(t *testing.T) {
		// prepare Jenkins
		jenkinsChangeID := os.Getenv("CHANGE_ID")
		jenkinsGitCommit := os.Getenv("GIT_COMMIT")
		jenkinsChangeBranchName := os.Getenv("BRANCH_NAME")
		jenkinsChangeTargetName := os.Getenv("CHANGE_TARGET")
		jenkinsURL := os.Getenv("JENKINS_URL")
		restoreJenkinsFn := func() {
			os.Setenv("CHANGE_ID", jenkinsChangeID)
			os.Setenv("GIT_COMMIT", jenkinsGitCommit)
			os.Setenv("BRANCH_NAME", jenkinsChangeBranchName)
			os.Setenv("CHANGE_TARGET", jenkinsChangeTargetName)
			os.Setenv("JENKINS_URL", jenkinsURL)
		}

		testSha := "0123456"
		testBranch := "mybranch"

		t.Run("Running for Branches", func(t *testing.T) {
			os.Setenv("JENKINS_URL", "http://jenkins.local")
			os.Setenv("GIT_COMMIT", testSha)
			os.Setenv("CHANGE_ID", "")
			os.Setenv("CHANGE_TARGET", "")
			os.Setenv("BRANCH_NAME", testBranch)
			defer restoreJenkinsFn()

			gitCtx := checkGiContext()
			assert.Equal(t, testSha, gitCtx.Commit)
			assert.Equal(t, testBranch, gitCtx.Branch)
			assert.Equal(t, testBranch, gitCtx.GetTargetBranch())
			assert.Equal(t, "Jenkins", gitCtx.Provider)
			assert.False(t, gitCtx.ChangeRequest)
		})

		t.Run("Running for Pull Requests", func(t *testing.T) {
			os.Setenv("JENKINS_URL", "http://jenkins.local")
			os.Setenv("GIT_COMMIT", testSha)
			os.Setenv("CHANGE_ID", "PR-123")
			os.Setenv("CHANGE_TARGET", "main")
			os.Setenv("BRANCH_NAME", testBranch)
			defer restoreJenkinsFn()

			gitCtx := checkGiContext()
			assert.Equal(t, testSha, gitCtx.Commit)
			assert.Equal(t, testBranch, gitCtx.Branch)
			assert.Equal(t, "main", gitCtx.GetTargetBranch())
			assert.Equal(t, "Jenkins", gitCtx.Provider)
			assert.True(t, gitCtx.ChangeRequest)
		})
	})

	t.Run("Gitlab", func(t *testing.T) {
		if originalGithubSha != "" {
			t.Skip("Tests skipped when running on Github")
		}

		// prepare Gitlab
		gitlabRefName := os.Getenv("CI_COMMIT_REF_NAME")
		originalSourceBranchSha := os.Getenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA")
		originalTargetBranchName := os.Getenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME")
		restoreGitlabFn := func() {
			os.Setenv("CI_COMMIT_BRANCH", originalGitlabBranch)
			os.Setenv("CI_COMMIT_REF_NAME", gitlabRefName)
			os.Setenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA", originalSourceBranchSha)
			os.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", originalTargetBranchName)
		}

		t.Run("Running for Branches", func(t *testing.T) {
			os.Setenv("CI_COMMIT_BRANCH", "branch")
			os.Setenv("CI_COMMIT_REF_NAME", "branch")
			os.Setenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA", "0123456")
			os.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "main")
			defer restoreGitlabFn()

			gitCtx := checkGiContext()
			assert.Equal(t, "0123456", gitCtx.Commit)
			assert.Equal(t, "branch", gitCtx.Branch)
			assert.Equal(t, "branch", gitCtx.GetTargetBranch())
			assert.Equal(t, "Gitlab", gitCtx.Provider)
			assert.False(t, gitCtx.ChangeRequest)
		})

		t.Run("Running for Merge Requests", func(t *testing.T) {
			os.Setenv("CI_COMMIT_REF_NAME", "branch")
			os.Setenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA", "0123456")
			os.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "main")
			defer restoreGitlabFn()

			gitCtx := checkGiContext()
			assert.Equal(t, "0123456", gitCtx.Commit)
			assert.Equal(t, "branch", gitCtx.Branch)
			assert.Equal(t, "main", gitCtx.GetTargetBranch())
			assert.Equal(t, "Gitlab", gitCtx.Provider)
			assert.True(t, gitCtx.ChangeRequest)
		})
	})

	t.Run("Local machine", func(t *testing.T) {
		if originalGithubSha != "" {
			t.Skip("Tests skipped when running on Github")
		}

		t.Run("Running with TARGET_BRANCH", func(t *testing.T) {
			os.Setenv("BRANCH", "foo")
			os.Setenv("TARGET_BRANCH", "main")
			defer os.Unsetenv("TARGET_BRANCH")
			defer os.Unsetenv("BRANCH")

			gitCtx := checkGiContext()
			assert.Equal(t, "", gitCtx.Commit)
			assert.Equal(t, "foo", gitCtx.Branch)
			assert.Equal(t, "main", gitCtx.GetTargetBranch())
			assert.Equal(t, "", gitCtx.Provider)
			assert.True(t, gitCtx.ChangeRequest)
		})

		t.Run("Running without TARGET_BRANCH", func(t *testing.T) {
			os.Setenv("BRANCH", "foo")
			defer os.Unsetenv("BRANCH")

			gitCtx := checkGiContext()
			assert.Equal(t, "", gitCtx.Commit)
			assert.Equal(t, "foo", gitCtx.Branch)
			assert.Equal(t, "foo", gitCtx.GetTargetBranch())
			assert.Equal(t, "", gitCtx.Provider)
			assert.False(t, gitCtx.ChangeRequest)
		})
	})

	t.Run("Empty SCM context", func(t *testing.T) {
		gitCtx := checkGiContext()
		assert.Nil(t, gitCtx)
	})
}

func TestGit_ContributeAttributesCloneOptions(t *testing.T) {
	os.Setenv("BRANCH", "master") // master branch is the base branch for the fake repository (octocat/hello-world)
	defer func() {
		os.Unsetenv("BRANCH")
	}()

	scm := NewFakeGitRepo(t, WithCloneOptions(CloneOptionsRequest{Depth: 1})).read()
	if scm == nil {
		t.FailNow()
	}

	atts := scm.contributeAttributes()

	// shallow clone depth is 3
	assert.Condition(t, func() bool { return keyExistsWithIntValue(t, atts, GitCloneDepth, 3) }, "should be set as scm.git.clone.depth=3. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExistsWithBoolValue(t, atts, GitCloneShallow, true) }, "should be set as scm.git.clone.shallow=true. Attributes: %v", atts)
}

func TestGit_ContributeAttributesForChangeRequests(t *testing.T) {
	branchName := "this-is-a-test-branch"
	os.Setenv("BRANCH", branchName)
	os.Setenv("TARGET_BRANCH", "master") // master branch is the base branch for the fake repository (octocat/hello-world)
	defer func() {
		os.Unsetenv("TARGET_BRANCH")
		os.Unsetenv("BRANCH")
	}()

	scm := NewFakeGitRepo(t, WithCloneOptions(CloneOptionsRequest{})).withBranch(branchName).addingFile("TEST-sample2.xml").removingFile("README").withCommit("This is a test commit").read()
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

	assert.Condition(t, func() bool { return keyExistsWithIntValue(t, atts, GitCloneDepth, 0) }, "should be set as scm.git.clone.depth=0. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExistsWithBoolValue(t, atts, GitCloneShallow, false) }, "should be set as scm.git.clone.shallow=false. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmBaseRef, "master") }, "should be set as scm.baseRef. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmBranch, branchName) }, "should be set as scm.branch. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmType, "git") }, "Git should be set as scm.type. Attributes: %v", atts)
	assert.Condition(t, func() bool {
		return keyExistsWithValue(t, atts, ScmRepository, "https://github.com/octocat/hello-world")
	}, "Remote should be set as scm.repository. Attributes: %v", atts)
}

func TestGit_ContributeAttributesForBranches(t *testing.T) {
	os.Setenv("BRANCH", "master") // master branch is the base branch for the fake repository (octocat/hello-world)
	scm := NewFakeGitRepo(t, WithCloneOptions(CloneOptionsRequest{})).read()
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

	assert.Condition(t, func() bool { return keyExistsWithIntValue(t, atts, GitCloneDepth, 0) }, "should be set as scm.git.clone.depth=0. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExistsWithBoolValue(t, atts, GitCloneShallow, false) }, "should be set as scm.git.clone.shallow=false. Attributes: %v", atts)
	assert.Condition(t, func() bool { return !keyExistsWithValue(t, atts, ScmBaseRef, "master") }, "should be set as scm.baseRef. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmBranch, "master") }, "Branch should be set as scm.branch. Attributes: %v", atts)
	assert.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmType, "git") }, "Git should be set as scm.type. Attributes: %v", atts)
	assert.Condition(t, func() bool {
		return keyExistsWithValue(t, atts, ScmRepository, "https://github.com/octocat/hello-world")
	}, "Remote should be set as scm.repository. Attributes: %v", atts)
}

func TestGit_ContributeCommitters(t *testing.T) {
	os.Setenv("TARGET_BRANCH", "master")
	defer os.Unsetenv("TARGET_BRANCH")

	scm := NewFakeGitRepo(t, WithCloneOptions(CloneOptionsRequest{})).withBranch("this-is-a-test-branch").addingFile("TEST-sample2.xml").withCommit("This is a test commit").read()
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

	scm := NewFakeGitRepo(t, WithCloneOptions(CloneOptionsRequest{})).withBranch("this-is-a-test-branch").addingFile("TEST-sample2.xml").removingFile("README").withCommit("This is a test commit").read()
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

	scm := NewFakeGitRepo(t, WithCloneOptions(CloneOptionsRequest{})).withBranch("this-is-a-test-branch").addingFile("TEST-sample2.xml").removingFile("README").withCommit("This is a test commit").read()
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
	scm := NewFakeGitRepo(t, WithCloneOptions(CloneOptionsRequest{})).read()
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

func keyExistsWithBoolValue(t *testing.T, attributes []attribute.KeyValue, key string, value ...bool) bool {
	for _, att := range attributes {
		if string(att.Key) == key {
			for _, v := range value {
				val := att.Value.AsBoolSlice()
				if len(val) == 0 {
					return v == att.Value.AsBool()
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
