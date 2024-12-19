package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckGitContext(t *testing.T) {
	t.Run("Github", func(t *testing.T) {
		// Disable Local
		t.Setenv("BRANCH", "")

		testSha := "0123456"
		testBaseRef := "main"
		testHeadRef := "feature/pr-23"

		t.Run("Running for Branches", func(t *testing.T) {
			t.Setenv("GITHUB_SHA", testSha)
			t.Setenv("GITHUB_REF_NAME", testHeadRef)
			t.Setenv("GITHUB_BASE_REF", "") // only for pull requests
			t.Setenv("GITHUB_HEAD_REF", "") // only for pull requests

			gitCtx := checkGitContext()
			require.Equal(t, testSha, gitCtx.Commit)
			require.Equal(t, testHeadRef, gitCtx.Branch)
			require.Equal(t, testHeadRef, gitCtx.GetTargetBranch())
			require.Equal(t, "Github", gitCtx.Provider)
			require.False(t, gitCtx.ChangeRequest)
		})

		t.Run("Running for Pull Requests", func(t *testing.T) {
			t.Setenv("GITHUB_SHA", testSha)
			t.Setenv("GITHUB_REF_NAME", testHeadRef)
			t.Setenv("GITHUB_BASE_REF", testBaseRef)
			t.Setenv("GITHUB_HEAD_REF", testHeadRef)

			gitCtx := checkGitContext()
			require.Equal(t, testSha, gitCtx.Commit)
			require.Equal(t, testHeadRef, gitCtx.Branch)
			require.Equal(t, testBaseRef, gitCtx.GetTargetBranch())
			require.Equal(t, "Github", gitCtx.Provider)
			require.True(t, gitCtx.ChangeRequest)
		})
	})

	t.Run("Jenkins", func(t *testing.T) {
		testSha := "0123456"
		testBranch := "mybranch"

		// Disable Local and Github
		t.Setenv("BRANCH", "")
		t.Setenv("GITHUB_SHA", "")

		t.Run("Running for Branches", func(t *testing.T) {
			t.Setenv("JENKINS_URL", "http://jenkins.local")
			t.Setenv("GIT_COMMIT", testSha)
			t.Setenv("CHANGE_ID", "")
			t.Setenv("CHANGE_TARGET", "")
			t.Setenv("BRANCH_NAME", testBranch)

			gitCtx := checkGitContext()
			require.Equal(t, testSha, gitCtx.Commit)
			require.Equal(t, testBranch, gitCtx.Branch)
			require.Equal(t, testBranch, gitCtx.GetTargetBranch())
			require.Equal(t, "Jenkins", gitCtx.Provider)
			require.False(t, gitCtx.ChangeRequest)
		})

		t.Run("Running for Pull Requests", func(t *testing.T) {
			t.Setenv("JENKINS_URL", "http://jenkins.local")
			t.Setenv("GIT_COMMIT", testSha)
			t.Setenv("CHANGE_ID", "PR-123")
			t.Setenv("CHANGE_TARGET", "main")
			t.Setenv("BRANCH_NAME", testBranch)

			gitCtx := checkGitContext()
			require.Equal(t, testSha, gitCtx.Commit)
			require.Equal(t, testBranch, gitCtx.Branch)
			require.Equal(t, "main", gitCtx.GetTargetBranch())
			require.Equal(t, "Jenkins", gitCtx.Provider)
			require.True(t, gitCtx.ChangeRequest)
		})
	})

	t.Run("Gitlab", func(t *testing.T) {
		// Disable Local, Github and Jenkins
		t.Setenv("BRANCH", "")
		t.Setenv("GITHUB_SHA", "")
		t.Setenv("JENKINS_URL", "")

		t.Run("Running for Branches", func(t *testing.T) {
			t.Setenv("CI_COMMIT_BRANCH", "branch")
			t.Setenv("CI_COMMIT_REF_NAME", "branch")
			t.Setenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA", "0123456")
			t.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "main")

			gitCtx := checkGitContext()
			require.Equal(t, "0123456", gitCtx.Commit)
			require.Equal(t, "branch", gitCtx.Branch)
			require.Equal(t, "branch", gitCtx.GetTargetBranch())
			require.Equal(t, "Gitlab", gitCtx.Provider)
			require.False(t, gitCtx.ChangeRequest)
		})

		t.Run("Running for Merge Requests", func(t *testing.T) {
			t.Setenv("CI_COMMIT_REF_NAME", "branch")
			t.Setenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA", "0123456")
			t.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "main")

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
			t.Setenv("BRANCH", "foo")
			t.Setenv("TARGET_BRANCH", "main")

			gitCtx := checkGitContext()
			require.Equal(t, "", gitCtx.Commit)
			require.Equal(t, "foo", gitCtx.Branch)
			require.Equal(t, "main", gitCtx.GetTargetBranch())
			require.Equal(t, "", gitCtx.Provider)
			require.True(t, gitCtx.ChangeRequest)
		})

		t.Run("Running without TARGET_BRANCH", func(t *testing.T) {
			t.Setenv("BRANCH", "foo")

			gitCtx := checkGitContext()
			require.Equal(t, "", gitCtx.Commit)
			require.Equal(t, "foo", gitCtx.Branch)
			require.Equal(t, "foo", gitCtx.GetTargetBranch())
			require.Equal(t, "", gitCtx.Provider)
			require.False(t, gitCtx.ChangeRequest)
		})
	})

	t.Run("Empty SCM context", func(t *testing.T) {
		// Disable Local, Github, Jenkins and Gitlab
		t.Setenv("BRANCH", "")
		t.Setenv("GITHUB_SHA", "")
		t.Setenv("JENKINS_URL", "")
		t.Setenv("CI_COMMIT_BRANCH", "")

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
