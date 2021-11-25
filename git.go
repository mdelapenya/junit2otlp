package main

import (
	"fmt"
	"os"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
)

type GitScm struct {
	repositoryPath string
}

// calculateCommits this method calculates the commits between current branch (HEAD) and a target branch.
// - The target branch has to be set as the TARGET_BRANCH environment variable
// - HEAD branch must be a valid branch in the git repository
func calculateCommits(repository *git.Repository) (*object.Commit, *object.Commit, error) {
	targetBranchEnv := os.Getenv("TARGET_BRANCH")
	if targetBranchEnv == "" {
		return nil, nil, fmt.Errorf("not processing committers because we are not able to calculate the target branch. Please set the TARGET_BRANCH variable with the name of the branch where you want to merge current branch")
	}

	targetBranch, err := repository.Branch(targetBranchEnv)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "not able to retrieve the %s TARGET_BRANCH: %v", targetBranchEnv, err)
	}

	targetRef, err := repository.ResolveRevision(plumbing.Revision(targetBranch.Merge))
	if err != nil {
		return nil, nil, errors.Wrapf(err, "not able to retrieve ref from TARGET_BRANCH: %v", err)
	}

	targetCommit, err := repository.CommitObject(*targetRef)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "not able to retrieve commit from TARGET_BRANCH: %v", err)
	}

	headRef, err := repository.Head()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "not able to retrieve ref from HEAD: %v", err)
	}

	headCommit, err := repository.CommitObject(headRef.Hash())
	if err != nil {
		return nil, nil, errors.Wrapf(err, "not able to retrieve commit from HEAD: %v", err)
	}

	return headCommit, targetCommit, nil
}

// contributeAttributes this method never fails, returning the current state of the contributed attributes
// at the moment of the failure
func (scm *GitScm) contributeAttributes() []attribute.KeyValue {
	repository, err := scm.openLocalRepository()
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
	gitAttributes = append(gitAttributes, attribute.Key(ScmRepository).StringSlice(origin.Config().URLs))

	branch, err := repository.Head()
	if err != nil {
		return gitAttributes
	}
	gitAttributes = append(gitAttributes, attribute.Key(ScmBranch).String(branch.Name().String()))

	contributions := []func(*git.Repository) ([]attribute.KeyValue, error){
		contributeCommitters,
	}

	for _, contribution := range contributions {
		contributtedAttributes, err := contribution(repository)
		if err != nil {
			fmt.Printf(">> not contributing attributes: %v", err)
			continue
		}

		gitAttributes = append(gitAttributes, contributtedAttributes...)
	}

	return gitAttributes
}

// contributeCommitters this method calculates the commits between current branch (HEAD) and a target branch.
// - The target branch has to be set as the TARGET_BRANCH environment variable
// - HEAD branch must be a valid branch in the git repository
// The algorithm will look for the first ancestor between HEAD and the TARGET_BRANCH, and will iterate through
// the list of commits, storing the author and the committer for each commit, contributing an array of Strings
// attribute including the email of the author/commiter.
// This method will return the current state of the contributed attributes at the moment of an eventual failure.
func contributeCommitters(repository *git.Repository) (attributes []attribute.KeyValue, outError error) {
	attributes = []attribute.KeyValue{}

	headCommit, targetCommit, err := calculateCommits(repository)
	if err != nil {
		outError = err
		return
	}

	commits, err := headCommit.MergeBase(targetCommit)
	if err != nil {
		outError = errors.Wrapf(err, "not able to find a common ancestor between HEAD and TARGET_BRANCH: %v", err)
		return
	}

	if len(commits) == 0 {
		outError = errors.Wrapf(err, "not able to find a common ancestor between HEAD and TARGET_BRANCH: %v", err)
		return
	}

	ancestor := commits[0]

	commitsIterator, err := repository.Log(&git.LogOptions{From: headCommit.Hash, Since: &ancestor.Author.When})
	if err != nil {
		outError = errors.Wrapf(err, "not able to retrieve commits between HEAD and TARGET_BRANCH: %v", err)
		return
	}

	authors := map[string]bool{}
	committers := map[string]bool{}

	commitsIterator.ForEach(func(c *object.Commit) error {
		authors[c.Author.Email] = true
		committers[c.Committer.Email] = true
		return nil
	})

	if len(authors) > 0 {
		attributes = append(attributes, attribute.Key(ScmAuthors).StringSlice(mapToArray(authors)))
	}

	if len(committers) > 0 {
		attributes = append(attributes, attribute.Key(ScmCommitters).StringSlice(mapToArray(committers)))
	}

	return
}

func mapToArray(m map[string]bool) []string {
	array := []string{}
	for k := range m {
		array = append(array, k)
	}

	return array
}

func (scm *GitScm) openLocalRepository() (*git.Repository, error) {
	repository, err := git.PlainOpen(scm.repositoryPath)
	if err != nil {
		return nil, err
	}

	return repository, nil
}
