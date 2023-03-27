package cmd_test

import (
	"errors"
	"os"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/concourse/concourse/cmd"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	ifritFakes "github.com/tedsuo/ifrit/fake_runner"
)

var _ = Describe("LoggingRunner", func() {

	var (
		runner                  ifrit.Runner
		fakeRunner              *ifritFakes.FakeRunner
		logger                  *lagertest.TestLogger
		fakeRunnerBlocker       chan interface{}
		fakeRunnerRunStubReturn error
		runnerError             chan error
		signals                 <-chan os.Signal
		ready                   chan<- struct{}
	)

	BeforeEach(func() {
		runnerError = make(chan error, 1)

		fakeRunnerBlocker = make(chan interface{})
		signals = make(chan os.Signal)
		ready = make(chan struct{})

		fakeRunner = new(ifritFakes.FakeRunner)
		fakeRunner.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
			<-fakeRunnerBlocker
			return fakeRunnerRunStubReturn
		}
		logger = lagertest.NewTestLogger("foo")
		runner = NewLoggingRunner(logger, fakeRunner)

		fakeRunnerRunStubReturn = errors.New("some-error")
		close(fakeRunnerBlocker)
	})

	JustBeforeEach(func() {
		go func() {
			runnerError <- runner.Run(signals, ready)
		}()
	})

	Describe("#Run", func() {
		It("logs the member name", func() {
			<-runnerError
			Expect(logger.LogMessages()).To(ContainElement("foo.logging-runner-exited"))
		})

		It("returns the child's return value", func() {
			err := <-runnerError
			Expect(err).To(Equal(fakeRunnerRunStubReturn))
		})

		It("invokes the child's Run with signals and ready", func() {
			<-runnerError
			Expect(fakeRunner.RunCallCount()).To(Equal(1))
			sig, read := fakeRunner.RunArgsForCall(0)

			Expect(sig).To(Equal(signals))
			Expect(read).To(Equal(ready))
		})
	})
})
