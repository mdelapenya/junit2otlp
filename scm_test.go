package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckGitContext(t *testing.T) {
	originalGithubSha := os.Getenv("GITHUB_SHA")
	originalGitlabBranch := os.Getenv("CI_COMMIT_BRANCH")

	unsetGithub := func() {
		os.Setenv("GITHUB_SHA", "")
	}
	unsetLocal := func() {
		os.Setenv("BRANCH", "")
		os.Setenv("TARGET_BRANCH", "")
	}

	restoreSCMContextsFn := func() {
		os.Setenv("GITHUB_SHA", originalGithubSha)
		os.Setenv("CI_COMMIT_BRANCH", originalGitlabBranch)
	}

	t.Run("Github", func(t *testing.T) {
		unsetLocal()
		// Prepare Github
		originalBaseRef := os.Getenv("GITHUB_BASE_REF")
		originalHeadRef := os.Getenv("GITHUB_HEAD_REF")
		originalRefName := os.Getenv("GITHUB_REF_NAME")
		restoreGithubFn := func() {
			restoreSCMContextsFn()
			os.Setenv("GITHUB_BASE_REF", originalBaseRef)
			os.Setenv("GITHUB_HEAD_REF", originalHeadRef)
			os.Setenv("GITHUB_REF_NAME", originalRefName)
		}

		if originalGitlabBranch != "" {
			t.Skip("Tests skipped when running on Gitlab")
		}

		testSha := "0123456"
		testBaseRef := "main"
		testHeadRef := "feature/pr-23"
		if originalGithubSha != "" {
			testSha = originalGithubSha
		}

		t.Run("Running for Branches", func(t *testing.T) {
			unsetLocal()
			os.Setenv("GITHUB_SHA", testSha)
			os.Setenv("GITHUB_REF_NAME", testHeadRef)
			os.Setenv("GITHUB_BASE_REF", "") // only for pull requests
			os.Setenv("GITHUB_HEAD_REF", "") // only for pull requests
			defer restoreGithubFn()

			gitCtx := checkGitContext()
			require.Equal(t, testSha, gitCtx.Commit)
			require.Equal(t, testHeadRef, gitCtx.Branch)
			require.Equal(t, testHeadRef, gitCtx.GetTargetBranch())
			require.Equal(t, "Github", gitCtx.Provider)
			require.False(t, gitCtx.ChangeRequest)
		})

		t.Run("Running for Pull Requests", func(t *testing.T) {
			unsetLocal()
			os.Setenv("GITHUB_SHA", testSha)
			os.Setenv("GITHUB_REF_NAME", testHeadRef)
			os.Setenv("GITHUB_BASE_REF", testBaseRef)
			os.Setenv("GITHUB_HEAD_REF", testHeadRef)
			defer restoreGithubFn()

			gitCtx := checkGitContext()
			require.Equal(t, testSha, gitCtx.Commit)
			require.Equal(t, testHeadRef, gitCtx.Branch)
			require.Equal(t, testBaseRef, gitCtx.GetTargetBranch())
			require.Equal(t, "Github", gitCtx.Provider)
			require.True(t, gitCtx.ChangeRequest)
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
			restoreSCMContextsFn()
			os.Setenv("CHANGE_ID", jenkinsChangeID)
			os.Setenv("GIT_COMMIT", jenkinsGitCommit)
			os.Setenv("BRANCH_NAME", jenkinsChangeBranchName)
			os.Setenv("CHANGE_TARGET", jenkinsChangeTargetName)
			os.Setenv("JENKINS_URL", jenkinsURL)
		}

		testSha := "0123456"
		testBranch := "mybranch"

		t.Run("Running for Branches", func(t *testing.T) {
			unsetLocal()
			unsetGithub()
			os.Setenv("JENKINS_URL", "http://jenkins.local")
			os.Setenv("GIT_COMMIT", testSha)
			os.Setenv("CHANGE_ID", "")
			os.Setenv("CHANGE_TARGET", "")
			os.Setenv("BRANCH_NAME", testBranch)
			defer restoreJenkinsFn()

			gitCtx := checkGitContext()
			require.Equal(t, testSha, gitCtx.Commit)
			require.Equal(t, testBranch, gitCtx.Branch)
			require.Equal(t, testBranch, gitCtx.GetTargetBranch())
			require.Equal(t, "Jenkins", gitCtx.Provider)
			require.False(t, gitCtx.ChangeRequest)
		})

		t.Run("Running for Pull Requests", func(t *testing.T) {
			unsetLocal()
			unsetGithub()
			os.Setenv("JENKINS_URL", "http://jenkins.local")
			os.Setenv("GIT_COMMIT", testSha)
			os.Setenv("CHANGE_ID", "PR-123")
			os.Setenv("CHANGE_TARGET", "main")
			os.Setenv("BRANCH_NAME", testBranch)
			defer restoreJenkinsFn()

			gitCtx := checkGitContext()
			require.Equal(t, testSha, gitCtx.Commit)
			require.Equal(t, testBranch, gitCtx.Branch)
			require.Equal(t, "main", gitCtx.GetTargetBranch())
			require.Equal(t, "Jenkins", gitCtx.Provider)
			require.True(t, gitCtx.ChangeRequest)
		})
	})

	t.Run("Gitlab", func(t *testing.T) {
		// prepare Gitlab
		gitlabRefName := os.Getenv("CI_COMMIT_REF_NAME")
		originalSourceBranchSha := os.Getenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA")
		originalTargetBranchName := os.Getenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME")
		restoreGitlabFn := func() {
			restoreSCMContextsFn()
			os.Setenv("CI_COMMIT_REF_NAME", gitlabRefName)
			os.Setenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA", originalSourceBranchSha)
			os.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", originalTargetBranchName)
		}

		t.Run("Running for Branches", func(t *testing.T) {
			unsetLocal()
			unsetGithub()
			os.Setenv("CI_COMMIT_BRANCH", "branch")
			os.Setenv("CI_COMMIT_REF_NAME", "branch")
			os.Setenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA", "0123456")
			os.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "main")
			defer restoreGitlabFn()

			gitCtx := checkGitContext()
			require.Equal(t, "0123456", gitCtx.Commit)
			require.Equal(t, "branch", gitCtx.Branch)
			require.Equal(t, "branch", gitCtx.GetTargetBranch())
			require.Equal(t, "Gitlab", gitCtx.Provider)
			require.False(t, gitCtx.ChangeRequest)
		})

		t.Run("Running for Merge Requests", func(t *testing.T) {
			unsetLocal()
			unsetGithub()
			os.Setenv("CI_COMMIT_REF_NAME", "branch")
			os.Setenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA", "0123456")
			os.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "main")
			defer restoreGitlabFn()

			gitCtx := checkGitContext()
			require.Equal(t, "0123456", gitCtx.Commit)
			require.Equal(t, "branch", gitCtx.Branch)
			require.Equal(t, "main", gitCtx.GetTargetBranch())
			require.Equal(t, "Gitlab", gitCtx.Provider)
			require.True(t, gitCtx.ChangeRequest)
		})
	})

	t.Run("Local machine", func(t *testing.T) {
		t.Run("Running with TARGET_BRANCH", func(t *testing.T) {
			unsetGithub()
			os.Setenv("BRANCH", "foo")
			os.Setenv("TARGET_BRANCH", "main")
			defer os.Unsetenv("TARGET_BRANCH")
			defer os.Unsetenv("BRANCH")
			defer restoreSCMContextsFn()

			gitCtx := checkGitContext()
			require.Equal(t, "", gitCtx.Commit)
			require.Equal(t, "foo", gitCtx.Branch)
			require.Equal(t, "main", gitCtx.GetTargetBranch())
			require.Equal(t, "", gitCtx.Provider)
			require.True(t, gitCtx.ChangeRequest)
		})

		t.Run("Running without TARGET_BRANCH", func(t *testing.T) {
			unsetGithub()
			os.Setenv("BRANCH", "foo")
			defer os.Unsetenv("BRANCH")
			defer restoreSCMContextsFn()

			gitCtx := checkGitContext()
			require.Equal(t, "", gitCtx.Commit)
			require.Equal(t, "foo", gitCtx.Branch)
			require.Equal(t, "foo", gitCtx.GetTargetBranch())
			require.Equal(t, "", gitCtx.Provider)
			require.False(t, gitCtx.ChangeRequest)
		})
	})

	t.Run("Empty SCM context", func(t *testing.T) {
		unsetLocal()
		unsetGithub()
		defer restoreSCMContextsFn()

		gitCtx := checkGitContext()
		require.Nil(t, gitCtx)
	})
}

func TestGetScm(t *testing.T) {
	t.Run("This project uses Git", func(t *testing.T) {
		scm := GetScm(getDefaultwd())
		switch scm.(type) {
		case *GitScm:
			// NOOP
		default:
			t.Error()
		}
	})

	t.Run("This project does not use Git", func(t *testing.T) {
		scm := GetScm(t.TempDir())

		require.Nil(t, scm, "The directory should not contain a .git directory")
	})
}

func TestGetTargetBranch(t *testing.T) {
	t.Run("For change-requests it must return target branch", func(t *testing.T) {
		ctx := &ScmContext{
			ChangeRequest: true,
			TargetBranch:  "target",
			Branch:        "branch",
		}

		require.Equal(t, "target", ctx.GetTargetBranch())
	})

	t.Run("For branches it must return branch", func(t *testing.T) {
		ctx := &ScmContext{
			ChangeRequest: false,
			TargetBranch:  "target",
			Branch:        "branch",
		}

		require.Equal(t, "branch", ctx.GetTargetBranch())
	})
}
