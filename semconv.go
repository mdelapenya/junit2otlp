package main

const (
	Junit2otlp = "junit2otlp"

	// git keys
	GitAdditions     = "scm.git.additions"
	GitCloneDepth    = "scm.git.clone.depth"
	GitCloneShallow  = "scm.git.clone.shallow"
	GitDeletions     = "scm.git.deletions"
	GitModifiedFiles = "scm.git.files.modified"

	// scm keys
	ScmAuthors    = "scm.authors"
	ScmBranch     = "scm.branch"
	ScmCommitters = "scm.committers"
	ScmProvider   = "scm.provider"
	ScmRepository = "scm.repository"
	ScmType       = "scm.type"

	// suite keys
	FailedTestsCount  = "tests.failed"
	ErrorTestsCount   = "tests.error"
	PassedTestsCount  = "tests.passed"
	SkippedTestsCount = "tests.skipped"
	TestsDuration     = "tests.duration"
	TestsSystemErr    = "tests.systemerr"
	TestsSystemOut    = "tests.systemout"
	TotalTestsCount   = "tests.total"

	// test keys
	TestClassName = "test.classname"
	TestDuration  = "test.duration"
	TestError     = "test.error"
	TestMessage   = "test.message"
	TestStatus    = "test.status"
	TestSystemErr = "test.systemerr"
	TestSystemOut = "test.systemout"
)
