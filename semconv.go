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
	ScmBaseRef    = "scm.baseRef"
	ScmBranch     = "scm.branch"
	ScmCommitters = "scm.committers"
	ScmProvider   = "scm.provider"
	ScmRepository = "scm.repository"
	ScmType       = "scm.type"

	// suite keys
	FailedTestsCount  = "tests.suite.failed"
	ErrorTestsCount   = "tests.suite.error"
	PassedTestsCount  = "tests.suite.passed"
	SkippedTestsCount = "tests.suite.skipped"
	TestsDuration     = "tests.suite.duration"
	TestsSuiteName    = "tests.suite.suitename"
	TestsSystemErr    = "tests.suite.systemerr"
	TestsSystemOut    = "tests.suite.systemout"
	TotalTestsCount   = "tests.suite.total"

	// test keys
	TestClassName = "tests.case.classname"
	TestDuration  = "tests.case.duration"
	TestError     = "tests.case.error"
	TestMessage   = "tests.case.message"
	TestStatus    = "tests.case.status"
	TestSystemErr = "tests.case.systemerr"
	TestSystemOut = "tests.case.systemout"
)
