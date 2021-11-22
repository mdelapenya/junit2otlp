package main

const (
	Junit2otlp = "junit2otlp"

	// scm keys
	ScmBranch     = "scm.branch"
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
	TestMessage   = "test.message"
	TestStatus    = "test.status"
	TestSystemErr = "test.systemerr"
	TestSystemOut = "test.systemout"
)
