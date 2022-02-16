package main

import (
	"os"
	"path"
)

type Scm interface {
	OTELAttributesContributor
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
