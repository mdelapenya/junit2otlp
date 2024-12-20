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
	"github.com/stretchr/testify/require"
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

func TestGit_ContributeAttributesCloneOptions(t *testing.T) {
	t.Setenv("GITHUB_SHA", "")

	type testData struct {
		provider string
		env      map[string]string
	}

	tests := []testData{
		{
			provider: "local",
			env: map[string]string{
				"BRANCH": "master", // master branch is the base branch for the fake repository (octocat/hello-world)
			},
		},
		{
			provider: "jenkins",
			env: map[string]string{
				"JENKINS_URL":   "http://local.jenkins.org",
				"BRANCH_NAME":   "master",
				"GIT_COMMIT":    "HEAD",
				"CHANGE_TARGET": "master", // master branch is the base branch for the fake repository (octocat/hello-world)
			},
		},
	}

	runTests := func(t *testing.T, td testData) {
		t.Run(td.provider, func(t *testing.T) {
			for k, v := range td.env {
				t.Setenv(k, v)
			}

			scm := NewFakeGitRepo(t, WithCloneOptions(CloneOptionsRequest{Depth: 1})).read()
			if scm == nil {
				t.FailNow()
			}

			atts := scm.contributeAttributes()

			// shallow clone depth is 3
			require.Condition(t, func() bool { return keyExistsWithIntValue(t, atts, GitCloneDepth, 3) }, "should be set as scm.git.clone.depth=3. Attributes: %v", atts)
			require.Condition(t, func() bool { return keyExistsWithBoolValue(t, atts, GitCloneShallow, true) }, "should be set as scm.git.clone.shallow=true. Attributes: %v", atts)
		})
	}

	for _, td := range tests {
		runTests(t, td)
	}
}

func TestGit_ContributeAttributesForChangeRequests(t *testing.T) {
	t.Setenv("GITHUB_SHA", "")

	type testData struct {
		provider string
		env      map[string]string
	}

	branchName := "feature/this-is-a-test-branch"

	tests := []testData{
		{
			provider: "local",
			env: map[string]string{
				"BRANCH":        branchName,
				"TARGET_BRANCH": "master", // master branch is the base branch for the fake repository (octocat/hello-world)
			},
		},
		{
			provider: "jenkins",
			env: map[string]string{
				"JENKINS_URL":   "http://local.jenkins.org",
				"BRANCH_NAME":   branchName,
				"CHANGE_ID":     branchName,
				"GIT_COMMIT":    "HEAD",
				"CHANGE_TARGET": "master", // master branch is the base branch for the fake repository (octocat/hello-world)
			},
		},
	}

	runTests := func(t *testing.T, td testData) {
		t.Run(td.provider, func(t *testing.T) {
			for k, v := range td.env {
				t.Setenv(k, v)
			}

			scm := NewFakeGitRepo(t, WithCloneOptions(CloneOptionsRequest{})).withBranch(branchName).addingFile("TEST-sample2.xml").removingFile("README").withCommit("This is a test commit").read()
			if scm == nil {
				t.FailNow()
			}

			atts := scm.contributeAttributes()

			require.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmAuthors, "author@test.com") }, "Authors should be set as scm.authors. Attributes: %v", atts)
			require.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmCommitters, "committer@test.com") }, "Committers should be set as scm.committers. Attributes: %v", atts)

			if scm.changeRequest {
				// we are adding 1 file with 202 lines, and we are deleting 1 file with 1 line
				require.Condition(t, func() bool { return keyExistsWithIntValue(t, atts, GitAdditions, 202) }, "Additions should be set as scm.git.additions. Attributes: %v", atts)
				require.Condition(t, func() bool { return keyExistsWithIntValue(t, atts, GitDeletions, 1) }, "Deletions should be set as scm.git.deletions. Attributes: %v", atts)
				require.Condition(t, func() bool { return keyExistsWithIntValue(t, atts, GitModifiedFiles, 2) }, "Modified files should be set as scm.git.modified.files. Attributes: %v", atts)
			}

			require.Condition(t, func() bool { return keyExistsWithIntValue(t, atts, GitCloneDepth, 0) }, "should be set as scm.git.clone.depth=0. Attributes: %v", atts)
			require.Condition(t, func() bool { return keyExistsWithBoolValue(t, atts, GitCloneShallow, false) }, "should be set as scm.git.clone.shallow=false. Attributes: %v", atts)
			require.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmBaseRef, "master") }, "should be set as scm.baseRef. Attributes: %v", atts)
			require.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmBranch, branchName) }, "should be set as scm.branch. Attributes: %v", atts)
			require.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmType, "git") }, "Git should be set as scm.type. Attributes: %v", atts)
			require.Condition(t, func() bool {
				return keyExistsWithValue(t, atts, ScmRepository, "https://github.com/octocat/hello-world")
			}, "Remote should be set as scm.repository. Attributes: %v", atts)
		})
	}

	for _, td := range tests {
		runTests(t, td)
	}
}

func TestGit_ContributeAttributesForBranches(t *testing.T) {
	t.Setenv("GITHUB_SHA", "")

	type testData struct {
		provider string
		env      map[string]string
	}

	tests := []testData{
		{
			provider: "local",
			env: map[string]string{
				"BRANCH": "master", // master branch is the base branch for the fake repository (octocat/hello-world)
			},
		},
		{
			provider: "jenkins",
			env: map[string]string{
				"JENKINS_URL":   "http://local.jenkins.org",
				"BRANCH_NAME":   "master",
				"GIT_COMMIT":    "HEAD",
				"CHANGE_TARGET": "master", // master branch is the base branch for the fake repository (octocat/hello-world)
			},
		},
	}

	runTests := func(t *testing.T, td testData) {
		t.Run(td.provider, func(t *testing.T) {
			for k, v := range td.env {
				t.Setenv(k, v)
			}

			scm := NewFakeGitRepo(t, WithCloneOptions(CloneOptionsRequest{})).read()
			if scm == nil {
				t.FailNow()
			}

			atts := scm.contributeAttributes()

			require.Condition(t, func() bool { return !keyExists(t, atts, ScmAuthors) }, "Authors shouldn't be set as scm.authors. Attributes: %v", atts)
			require.Condition(t, func() bool { return !keyExists(t, atts, ScmCommitters) }, "Committers shouldn't be set as scm.committers. Attributes: %v", atts)

			if scm.changeRequest {
				require.Condition(t, func() bool { return !keyExists(t, atts, GitAdditions) }, "Additions shouldn't be as scm.git.additions. Attributes: %v", atts)
				require.Condition(t, func() bool { return !keyExists(t, atts, GitDeletions) }, "Deletions shouldn't be as scm.git.deletions. Attributes: %v", atts)
				require.Condition(t, func() bool { return !keyExists(t, atts, GitModifiedFiles) }, "Modified files shouldn't be as scm.git.modified.files. Attributes: %v", atts)
			}

			require.Condition(t, func() bool { return keyExistsWithIntValue(t, atts, GitCloneDepth, 0) }, "should be set as scm.git.clone.depth=0. Attributes: %v", atts)
			require.Condition(t, func() bool { return keyExistsWithBoolValue(t, atts, GitCloneShallow, false) }, "should be set as scm.git.clone.shallow=false. Attributes: %v", atts)
			require.Condition(t, func() bool { return !keyExistsWithValue(t, atts, ScmBaseRef, "master") }, "should be set as scm.baseRef. Attributes: %v", atts)
			require.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmBranch, "master") }, "Branch should be set as scm.branch. Attributes: %v", atts)
			require.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmType, "git") }, "Git should be set as scm.type. Attributes: %v", atts)
			require.Condition(t, func() bool {
				return keyExistsWithValue(t, atts, ScmRepository, "https://github.com/octocat/hello-world")
			}, "Remote should be set as scm.repository. Attributes: %v", atts)
		})
	}

	for _, td := range tests {
		runTests(t, td)
	}
}

func TestGit_ContributeCommitters(t *testing.T) {
	t.Setenv("GITHUB_SHA", "")

	branchName := "feature/this-is-a-test-branch"

	type testData struct {
		provider string
		env      map[string]string
	}

	tests := []testData{
		{
			provider: "local",
			env: map[string]string{
				"BRANCH":        branchName,
				"TARGET_BRANCH": "master", // master branch is the base branch for the fake repository (octocat/hello-world)
			},
		},
		{
			provider: "jenkins",
			env: map[string]string{
				"JENKINS_URL":   "http://local.jenkins.org",
				"CHANGE_ID":     branchName,
				"BRANCH_NAME":   branchName,
				"GIT_COMMIT":    "HEAD",
				"CHANGE_TARGET": "master", // master branch is the base branch for the fake repository (octocat/hello-world)
			},
		},
	}

	runTests := func(t *testing.T, td testData) {
		t.Run(td.provider, func(t *testing.T) {
			for k, v := range td.env {
				t.Setenv(k, v)
			}

			scm := NewFakeGitRepo(t, WithCloneOptions(CloneOptionsRequest{})).withBranch(branchName).addingFile("TEST-sample2.xml").withCommit("This is a test commit").read()
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

			require.Equal(t, 2, len(atts))
			// we are adding 1 file with 202 lines, and we are deleting 1 file with 1 line
			require.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmAuthors, "author@test.com") }, "Authors should be set as scm.authors. Attributes: %v", atts)
			require.Condition(t, func() bool { return keyExistsWithValue(t, atts, ScmCommitters, "committer@test.com") }, "Committers should be set as scm.committers. Attributes: %v", atts)
		})
	}

	for _, td := range tests {
		runTests(t, td)
	}
}

func TestGit_ContributeFilesAndLines(t *testing.T) {
	t.Setenv("GITHUB_SHA", "")

	branchName := "feature/this-is-a-test-branch"

	type testData struct {
		provider string
		env      map[string]string
	}

	tests := []testData{
		{
			provider: "local",
			env: map[string]string{
				"BRANCH":        branchName,
				"TARGET_BRANCH": "master", // master branch is the base branch for the fake repository (octocat/hello-world)
			},
		},
		{
			provider: "jenkins",
			env: map[string]string{
				"JENKINS_URL":   "http://local.jenkins.org",
				"CHANGE_ID":     branchName,
				"BRANCH_NAME":   branchName,
				"GIT_COMMIT":    "HEAD",
				"CHANGE_TARGET": "master", // master branch is the base branch for the fake repository (octocat/hello-world)
			},
		},
	}

	runTests := func(t *testing.T, td testData) {
		t.Run(td.provider, func(t *testing.T) {
			for k, v := range td.env {
				t.Setenv(k, v)
			}

			scm := NewFakeGitRepo(t, WithCloneOptions(CloneOptionsRequest{})).withBranch(branchName).addingFile("TEST-sample2.xml").removingFile("README").withCommit("This is a test commit").read()
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

			require.Equal(t, 3, len(atts))
			// we are adding 1 file with 202 lines, and we are deleting 1 file with 1 line
			require.Condition(t, func() bool { return keyExistsWithIntValue(t, atts, GitAdditions, 202) }, "Additions should be set as scm.git.additions. Attributes: %v", atts)
			require.Condition(t, func() bool { return keyExistsWithIntValue(t, atts, GitDeletions, 1) }, "Deletions should be set as scm.git.deletions. Attributes: %v", atts)
			require.Condition(t, func() bool { return keyExistsWithIntValue(t, atts, GitModifiedFiles, 2) }, "Modified files should be set as scm.git.modified.files. Attributes: %v", atts)
		})
	}

	for _, td := range tests {
		runTests(t, td)
	}
}

func TestGit_CalculateCommitsForChangeRequests(t *testing.T) {
	t.Setenv("GITHUB_SHA", "")

	branchName := "feature/this-is-a-test-branch"

	type testData struct {
		provider string
		env      map[string]string
	}

	tests := []testData{
		{
			provider: "local",
			env: map[string]string{
				"BRANCH":        branchName,
				"TARGET_BRANCH": "master", // master branch is the base branch for the fake repository (octocat/hello-world)
			},
		},
		{
			provider: "jenkins",
			env: map[string]string{
				"JENKINS_URL":   "http://local.jenkins.org",
				"CHANGE_ID":     branchName,
				"BRANCH_NAME":   branchName,
				"GIT_COMMIT":    "HEAD",
				"CHANGE_TARGET": "master", // master branch is the base branch for the fake repository (octocat/hello-world)
			},
		},
	}

	runTests := func(t *testing.T, td testData) {
		t.Run(td.provider, func(t *testing.T) {
			for k, v := range td.env {
				t.Setenv(k, v)
			}

			scm := NewFakeGitRepo(t, WithCloneOptions(CloneOptionsRequest{})).withBranch(branchName).addingFile("TEST-sample2.xml").removingFile("README").withCommit("This is a test commit").read()
			if scm == nil {
				t.FailNow()
			}

			headCommit, targetCommit, err := scm.calculateCommits()
			if err != nil {
				t.Error()
			}

			require.NotNil(t, headCommit)
			require.NotNil(t, targetCommit)
		})
	}

	for _, td := range tests {
		runTests(t, td)
	}
}

func TestGit_CalculateCommitsForBranches(t *testing.T) {
	t.Setenv("GITHUB_SHA", "")

	type testData struct {
		provider string
		env      map[string]string
	}

	tests := []testData{
		{
			provider: "local",
			env: map[string]string{
				"BRANCH": "master", // master branch is the base branch for the fake repository (octocat/hello-world)
			},
		},
		{
			provider: "jenkins",
			env: map[string]string{
				"JENKINS_URL":   "http://local.jenkins.org",
				"BRANCH_NAME":   "master",
				"GIT_COMMIT":    "HEAD",
				"CHANGE_TARGET": "master", // master branch is the base branch for the fake repository (octocat/hello-world)
			},
		},
	}

	runTests := func(t *testing.T, td testData) {
		t.Run(td.provider, func(t *testing.T) {
			for k, v := range td.env {
				t.Setenv(k, v)
			}

			scm := NewFakeGitRepo(t, WithCloneOptions(CloneOptionsRequest{})).read()
			if scm == nil {
				t.FailNow()
			}

			headCommit, targetCommit, err := scm.calculateCommits()
			if err != nil {
				t.Error()
			}

			require.Equal(t, headCommit, targetCommit)
		})
	}

	for _, td := range tests {
		runTests(t, td)
	}
}

func keyExists(t *testing.T, attributes []attribute.KeyValue, key string) bool {
	t.Helper()

	for _, att := range attributes {
		if string(att.Key) == key {
			return true
		}
	}

	return false
}

func keyExistsWithBoolValue(t *testing.T, attributes []attribute.KeyValue, key string, value ...bool) bool {
	t.Helper()

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
	t.Helper()

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
	t.Helper()

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
