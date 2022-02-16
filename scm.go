package main

import (
	"os"
	"path"
)

type Scm interface {
	OTELAttributesContributor
}

type ScmContext struct {
	Branch        string
	ChangeRequest bool
	Commit        string
	Provider      string
	TargetBranch  string
}

func FromGithub() *ScmContext {
	if os.Getenv("GITHUB_SHA") == "" {
		return nil
	}

	sha := os.Getenv("GITHUB_SHA")
	branchName := os.Getenv("GITHUB_REF_NAME")
	baseRef := os.Getenv("GITHUB_BASE_REF") // only present for pull requests on Github Actions
	headRef := os.Getenv("GITHUB_HEAD_REF") // only present for pull requests on Github Actions

	isChangeRequest := (baseRef != "" && headRef != "")

	return &ScmContext{
		ChangeRequest: isChangeRequest,
		Commit:        sha,
		Branch:        branchName,
		Provider:      "Github",
		TargetBranch:  baseRef,
	}
}

func FromGitlab() *ScmContext {
	if os.Getenv("CI_COMMIT_REF_NAME") == "" {
		return nil
	}

	sha := os.Getenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA")      // only present on merge requests on Gitlab CI
	commitBranch := os.Getenv("CI_COMMIT_BRANCH")               // only present on branches on Gitlab CI
	headRef := os.Getenv("CI_COMMIT_REF_NAME")                  // only present on branches on Gitlab CI
	baseRef := os.Getenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME") // only present on merge requests on Gitlab CI

	isChangeRequest := (commitBranch == "")

	return &ScmContext{
		ChangeRequest: isChangeRequest,
		Commit:        sha,
		Branch:        headRef,
		Provider:      "Gitlab",
		TargetBranch:  baseRef,
	}
}

func FromJenkins() *ScmContext {
	if os.Getenv("JENKINS_URL") == "" {
		return nil
	}

	isPR := os.Getenv("CHANGE_ID") != ""  // only present on multibranch pipelines on Jenkins
	headRef := os.Getenv("BRANCH_NAME")   // only present on multibranch pipelines on Jenkins
	sha := os.Getenv("GIT_COMMIT")        // only present on multibranch pipelines on Jenkins
	baseRef := os.Getenv("CHANGE_TARGET") // only present on multibranch pipelines on Jenkins

	if isPR {
		return &ScmContext{
			ChangeRequest: isPR,
			Commit:        sha,
			Branch:        headRef,
			Provider:      "Jenkins",
			TargetBranch:  baseRef,
		}
	} else {
		return &ScmContext{
			ChangeRequest: isPR,
			Commit:        sha,
			Branch:        headRef,
			Provider:      "Jenkins",
			TargetBranch:  headRef,
		}
	}

}

// GetScm checks if the underlying filesystem repository is a Git repository
// checking the existence of the .git directory in the current workspace
func GetScm(repoDir string) Scm {
	// if .git file exists
	_, err := os.Stat(path.Join(repoDir, ".git"))
	if os.IsNotExist(err) {
		return nil
	}

	// .git exists
	return NewGitScm(repoDir)
}
