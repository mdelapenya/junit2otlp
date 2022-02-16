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
