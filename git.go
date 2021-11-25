package main

import (
	"fmt"
	"os"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
)

type GitScm struct {
	baseRef        string
	headSha        string
	isRequest      bool // if the tool is evaluating a pull/merge request or a branch
	provider       string
	repository     *git.Repository
	repositoryPath string
}

func NewGitScm(repositoryPath string) *GitScm {
	scm := &GitScm{
		repositoryPath: repositoryPath,
	}

	repository, err := scm.openLocalRepository()
	if err != nil {
		return nil
	}

	scm.repository = repository

	headSha, baseRef, gitProvider, request := checkGitProvider()

	scm.headSha = headSha
	scm.baseRef = baseRef
	scm.isRequest = request
	scm.provider = gitProvider

	fmt.Printf(">> HEAD SHA: %s", headSha)
	fmt.Printf(">> BASE_REF: %s", baseRef)

	return scm
}

// calculateCommits this method calculates the commits between current branch (HEAD) and a target branch.
// - The target branch has to be set as the TARGET_BRANCH environment variable
// - HEAD branch must be a valid branch in the git repository
func (scm *GitScm) calculateCommits() (*object.Commit, *object.Commit, error) {
	targetBranch, err := scm.repository.Branch(scm.baseRef)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "not able to retrieve the %s TARGET_BRANCH: %v", scm.baseRef, err)
	}

	targetRef, err := scm.repository.ResolveRevision(plumbing.Revision(targetBranch.Merge))
	if err != nil {
		return nil, nil, errors.Wrapf(err, "not able to retrieve ref from TARGET_BRANCH: %v", err)
	}

	targetCommit, err := scm.repository.CommitObject(*targetRef)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "not able to retrieve commit from TARGET_BRANCH: %v", err)
	}

	var headRefSha plumbing.Hash
	if scm.headSha == "" {
		headRef, err := scm.repository.Head()
		if err != nil {
			return nil, nil, errors.Wrapf(err, "not able to retrieve ref from HEAD: %v", err)
		}

		headRefSha = headRef.Hash()
	} else {
		headRefSha = plumbing.NewHash(scm.headSha)
	}

	headCommit, err := scm.repository.CommitObject(headRefSha)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "not able to retrieve commit from HEAD: %v", err)
	}

	return headCommit, targetCommit, nil
}

// checkGitProvider identies the head sha and target branch from the environment variables that are
// populated from a Git provider, such as Github or Gitlab. If no proprietary env vars are set, then it will
// look up this tool-specific variable for the target branch.
func checkGitProvider() (string, string, string, bool) {
	// is Github?
	sha := os.Getenv("GITHUB_SHA")
	baseRef := os.Getenv("GITHUB_BASE_REF") // only present for pull requests on Github Actions
	headRef := os.Getenv("GITHUB_HEAD_REF") // only present for pull requests on Github Actions
	if sha != "" && baseRef != "" && headRef != "" {
		return sha, baseRef, "Github", true
	} else if sha != "" {
		return sha, "", "Github", false
	}

	// is Gitlab?
	commitBranch := os.Getenv("CI_COMMIT_BRANCH")              // only present on branches on Gitlab CI
	sha = os.Getenv("CI_MERGE_REQUEST_SOURCE_BRANCH_SHA")      // only present on merge requests on Gitlab CI
	baseRef = os.Getenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME") // only present on merge requests on Gitlab CI
	if sha != "" && baseRef != "" {
		return sha, baseRef, "Gitlab", commitBranch == ""
	}

	// in local branches, we are not in pull/merge requests
	baseRef = os.Getenv("TARGET_BRANCH")
	return "", baseRef, "", false
}

// contributeAttributes this method never fails, returning the current state of the contributed attributes
// at the moment of the failure
func (scm *GitScm) contributeAttributes() []attribute.KeyValue {
	// from now on, this is a Git repository
	gitAttributes := []attribute.KeyValue{
		attribute.Key(ScmType).String("git"),
	}

	if scm.provider != "" {
		gitAttributes = append(gitAttributes, attribute.Key(ScmProvider).String(scm.provider))
	}

	origin, err := scm.repository.Remote("origin")
	if err != nil {
		return gitAttributes
	}
	gitAttributes = append(gitAttributes, attribute.Key(ScmRepository).StringSlice(origin.Config().URLs))

	branch, err := scm.repository.Head()
	if err != nil {
		return gitAttributes
	}
	gitAttributes = append(gitAttributes, attribute.Key(ScmBranch).String(branch.Name().String()))

	headCommit, targetCommit, err := scm.calculateCommits()
	if err != nil {
		return gitAttributes
	}

	contributions := []func(*object.Commit, *object.Commit) ([]attribute.KeyValue, error){
		scm.contributeCommitters,
	}

	if scm.isRequest {
		// calculate modified lines for pull/merge requests
		contributions = append(contributions, scm.contributeFilesAndLines)
	}

	for _, contribution := range contributions {
		contributtedAttributes, err := contribution(headCommit, targetCommit)
		if err != nil {
			fmt.Printf(">> not contributing attributes: %v", err)
			continue
		}

		gitAttributes = append(gitAttributes, contributtedAttributes...)
	}

	return gitAttributes
}

// contributeCommitters this algorithm will look for the first ancestor between HEAD and the TARGET_BRANCH, and will iterate through
// the list of commits, storing the author and the committer for each commit, contributing an array of Strings
// attribute including the email of the author/commiter.
// This method will return the current state of the contributed attributes at the moment of an eventual failure.
func (scm *GitScm) contributeCommitters(headCommit *object.Commit, targetCommit *object.Commit) (attributes []attribute.KeyValue, outError error) {
	attributes = []attribute.KeyValue{}

	fmt.Printf(">>> HEAD commit: %v", headCommit)
	fmt.Printf(">>> TARGET commit: %v", targetCommit)

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

	when := ancestor.Author.When.Add(time.Millisecond * 1) // adding one millisecond to avoid including the ancestor in the log
	commitsIterator, err := scm.repository.Log(&git.LogOptions{From: headCommit.Hash, Since: &when})
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

// contributeFilesAndLines this algorithm will look for the first ancestor between HEAD and the TARGET_BRANCH, and will iterate through
// the list of commits, storing the modified files for each commit; for each modified file it will get the added and deleted lines.
// It will contribute an Integer attribute including number of modified files, including added and deleted lines in the changeset.
// This method will return the current state of the contributed attributes at the moment of an eventual failure.
func (scm *GitScm) contributeFilesAndLines(headCommit *object.Commit, targetCommit *object.Commit) (attributes []attribute.KeyValue, outError error) {
	attributes = []attribute.KeyValue{}

	headTree, err := headCommit.Tree()
	if err != nil {
		outError = errors.Wrapf(err, "not able to find a HEAD tree: %v", err)
		return
	}

	targetTree, err := targetCommit.Tree()
	if err != nil {
		outError = errors.Wrapf(err, "not able to find a TARGET_BRANCH tree: %v", err)
		return
	}

	patch, err := targetTree.Patch(headTree)
	if err != nil {
		outError = errors.Wrapf(err, "not able to find the pathc between HEAD and TARGET_BRANCH trees: %v", err)
		return
	}

	var changedFiles []string
	var additions int = 0
	var deletions int = 0
	for _, fileStat := range patch.Stats() {
		additions += fileStat.Addition
		deletions += fileStat.Deletion

		changedFiles = append(changedFiles, fileStat.Name)
	}

	attributes = append(attributes, attribute.Key(GitAdditions).Int(additions))
	attributes = append(attributes, attribute.Key(GitDeletions).Int(deletions))
	attributes = append(attributes, attribute.Key(GitModifiedFiles).Int(len(changedFiles)))

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
