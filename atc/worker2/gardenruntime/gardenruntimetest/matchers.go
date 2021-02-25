package gardenruntimetest

import (
	"errors"
	"testing/fstest"

	"github.com/concourse/baggageclaim"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

func HaveStrategy(strategy baggageclaim.Strategy) types.GomegaMatcher {
	return haveStrategyMatcher{strategy}
}

type haveStrategyMatcher struct {
	expected baggageclaim.Strategy
}

func (m haveStrategyMatcher) Match(actual interface{}) (bool, error) {
	volume, ok := actual.(*Volume)
	if !ok {
		return false, errors.New("expecting a *grt.Volume")
	}

	return StrategyEq(m.expected)(volume), nil
}
func (m haveStrategyMatcher) FailureMessage(actual interface{}) string {
	return format.Message(actual, "to have strategy", m.expected)
}
func (m haveStrategyMatcher) NegatedFailureMessage(actual interface{}) string {
	return format.Message(actual, "not to have strategy", m.expected)
}

func HaveContent(content fstest.MapFS) types.GomegaMatcher {
	return haveContentMatcher{content}
}

type haveContentMatcher struct {
	expected fstest.MapFS
}

func (m haveContentMatcher) Match(actual interface{}) (bool, error) {
	volume, ok := actual.(*Volume)
	if !ok {
		return false, errors.New("expecting a *grt.Volume")
	}

	return ContentEq(m.expected)(volume), nil
}
func (m haveContentMatcher) FailureMessage(actual interface{}) string {
	return format.Message(actual, "to have content", m.expected)
}
func (m haveContentMatcher) NegatedFailureMessage(actual interface{}) string {
	return format.Message(actual, "not to have content", m.expected)
}

func BePrivileged() types.GomegaMatcher {
	return bePrivilegedMatcher{true}
}

type bePrivilegedMatcher struct {
	expected bool
}

func (m bePrivilegedMatcher) Match(actual interface{}) (bool, error) {
	volume, ok := actual.(*Volume)
	if !ok {
		return false, errors.New("expecting a *grt.Volume")
	}

	return PrivilegedEq(m.expected)(volume), nil
}
func (m bePrivilegedMatcher) FailureMessage(actual interface{}) string {
	return format.Message(actual, "to be "+m.expectation())
}
func (m bePrivilegedMatcher) NegatedFailureMessage(actual interface{}) string {
	return format.Message(actual, "not to be "+m.expectation())
}

func (m bePrivilegedMatcher) expectation() string {
	if m.expected {
		return "privileged"
	}
	return "unprivileged"
}

func HaveHandle(handle string) types.GomegaMatcher {
	return haveHandleMatcher{handle}
}

type haveHandleMatcher struct {
	expected string
}

func (m haveHandleMatcher) Match(actual interface{}) (bool, error) {
	volume, ok := actual.(*Volume)
	if !ok {
		return false, errors.New("expecting a *grt.Volume")
	}

	return HandleEq(m.expected)(volume), nil
}
func (m haveHandleMatcher) FailureMessage(actual interface{}) string {
	return format.Message(actual, "to have handle "+m.expected)
}
func (m haveHandleMatcher) NegatedFailureMessage(actual interface{}) string {
	return format.Message(actual, "not to have handle "+m.expected)
}
