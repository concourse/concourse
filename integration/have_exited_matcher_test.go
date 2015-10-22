package integration_test

import (
	"fmt"
	"os/exec"
	"syscall"

	"github.com/onsi/gomega/format"
)

type HaveExitedMatcher struct {
	statusCode int

	actual   int
	expected int
}

func HaveExited(statusCode int) *HaveExitedMatcher {
	return &HaveExitedMatcher{statusCode: statusCode}
}

func (matcher *HaveExitedMatcher) Match(actual interface{}) (bool, error) {
	switch v := actual.(type) {
	case *exec.Cmd:
		matcher.actual = v.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
	default:
		return false, fmt.Errorf("unknown type: %T", actual)
	}

	matcher.expected = matcher.statusCode

	return matcher.actual == matcher.expected, nil
}

func (matcher *HaveExitedMatcher) FailureMessage(actual interface{}) string {
	return format.Message(matcher.actual, "To have exited with status code", matcher.expected)
}

func (matcher *HaveExitedMatcher) NegatedFailureMessage(actual interface{}) string {
	return format.Message(matcher.actual, "To not have exited with status code", matcher.expected)
}
