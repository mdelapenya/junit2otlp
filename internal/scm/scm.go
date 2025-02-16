package scm

import (
	"os"
	"path"

	"go.opentelemetry.io/otel/attribute"
)

type OTELAttributesContributor interface {
	ContributeAttributes() []attribute.KeyValue
}

type Scm interface {
	OTELAttributesContributor
}

// ScmContext represent the execution context in which the SCM is used
type ScmContext struct {
	// Branch the name of the branch, which will be calculated
	// reading the environment variables for each supported context
	Branch string
	// ChangeRequest if the SCM context is for a change request
	ChangeRequest bool
	// Commit the commit hash for the SCM context
	Commit string
	// Provider the provider of the SCM context: Github, Gitlab, Jenkins, Other, etc.
	Provider string
	// TargetBranch the name of the branch in the case the SCM context represents a
	// change request. In the case ChangeRequest is false, it won't be considered
	TargetBranch string
}

// checkGitContext identifies the head sha and target branch from the environment variables that are
// populated from a Git provider, such as Github or Gitlab. If no proprietary env vars are set, then it will
// look up this tool-specific variable for the target branch.
func checkGitContext() *ScmContext {
	// in local branches, we are not in pull/merge requests
	localContext := FromLocal()
	if localContext != nil {
		return localContext
	}

	// is Github?
	githubContext := FromGithub()
	if githubContext != nil {
		return githubContext
	}

	// is Jenkins?
	jenkinsContext := FromJenkins()
	if jenkinsContext != nil {
		return jenkinsContext
	}

	// is Gitlab?
	gitlabContext := FromGitlab()
	if gitlabContext != nil {
		return gitlabContext
	}

	// SCM context not supported
	return nil
}

// GetTargetBranch returns the target branch for change requests, or branches in any other case
func (ctx *ScmContext) GetTargetBranch() string {
	if ctx.ChangeRequest {
		return ctx.TargetBranch
	}

	return ctx.Branch
}

// FromGithub returns an SCM context for Github, reading the right environment variables, as described
// in their docs
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

// FromGitlab returns an SCM context for Gitlab, reading the right environment variables, as described
// in their docs
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

// FromJenkins returns an SCM context for Jenkins, reading the right environment variables, as described
// in their docs
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

// FromLocal returns an SCM context for local, using TARGET_BRANCH and BRANCH as the variables controlling
// if the SCM context represents a change request. BRANCH is mandatory, otherwise an empty context will be retrieved.
// If TARGET_BRANCH is not empty, it will represent a change request
func FromLocal() *ScmContext {
	baseRef := os.Getenv("TARGET_BRANCH")
	headRef := os.Getenv("BRANCH")
	if headRef == "" {
		return nil
	}

	isPR := (baseRef != "")

	return &ScmContext{
		ChangeRequest: isPR,
		Commit:        "",
		Branch:        headRef,
		Provider:      "",
		TargetBranch:  baseRef,
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
