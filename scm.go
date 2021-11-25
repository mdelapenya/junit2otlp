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
func GetScm() Scm {
	// if .git file exists
	workingDir, err := os.Getwd()
	if err != nil {
		return nil
	}

	_, err = os.Stat(path.Join(workingDir, ".git"))
	if os.IsNotExist(err) {
		return nil
	}

	// .git exists
	return &GitScm{
		repositoryPath: workingDir,
	}
}
