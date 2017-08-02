package concourse_test

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/concourse/go-concourse/concourse"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"testing"
)

func TestApi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Concourse Client Suite")
}

var (
	atcServer *ghttp.Server
	client    concourse.Client
	team      concourse.Team
	tracing   bool
)

var _ = BeforeEach(func() {
	atcServer = ghttp.NewServer()

	client = concourse.NewClient(
		atcServer.URL(),
		&http.Client{},
		tracing,
	)

	team = client.Team("some-team")
})

var _ = AfterEach(func() {
	atcServer.Close()
})

func Change(fn func() int) *changeMatcher {
	return &changeMatcher{
		fn: fn,
	}
}

type changeMatcher struct {
	fn     func() int
	amount int

	before int
	after  int
}

func (cm *changeMatcher) By(amount int) *changeMatcher {
	cm.amount = amount

	return cm
}

func (cm *changeMatcher) Match(actual interface{}) (success bool, err error) {
	cm.before = cm.fn()

	ac, ok := actual.(func())
	if !ok {
		return false, errors.New("expected a function")
	}

	ac()

	cm.after = cm.fn()

	return (cm.after - cm.before) == cm.amount, nil
}

func (cm *changeMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected value to change by %d but it changed from %d to %d", cm.amount, cm.before, cm.after)
}

func (cm *changeMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected value not to change by %d but it changed from %d to %d", cm.amount, cm.before, cm.after)
}
