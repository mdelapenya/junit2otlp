package main

import (
	git "github.com/go-git/go-git/v5"
	"go.opentelemetry.io/otel/attribute"
) // with go modules enabled (GO111MODULE=on or outside GOPATH)

type GitScm struct {
	repositoryPath string
}

// contributeOtelAttributes this method never fails, returning the current state of the contributed attributes
// at the moment of the failure
func (scm *GitScm) contributeOtelAttributes() []attribute.KeyValue {
	repository, err := git.PlainOpen(scm.repositoryPath)
	if err != nil {
		return []attribute.KeyValue{}
	}

	// from now on, this is a Git repository
	gitAttributes := []attribute.KeyValue{
		attribute.Key(ScmType).String("git"),
	}

	origin, err := repository.Remote("origin")
	if err != nil {
		return gitAttributes
	}
	gitAttributes = append(gitAttributes, attribute.Key(ScmRepository).String(origin.Config().URLs[0]))

	branch, err := repository.Head()
	if err != nil {
		return gitAttributes
	}
	gitAttributes = append(gitAttributes, attribute.Key(ScmBranch).String(branch.Name().String()))

	return gitAttributes
}
