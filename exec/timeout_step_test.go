package exec_test

import (
	"errors"
	"os"
	"time"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/exec"

	"github.com/concourse/atc/exec/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/tedsuo/ifrit"
)

var _ = Describe("Timeout Step", func() {
	var (
		fakeStepFactoryStep *fakes.FakeStepFactory

		runStep *fakes.FakeStep

		timeout StepFactory
		step    Step

		startStep chan error
		process   ifrit.Process

		timeoutDuration atc.Duration
	)

	BeforeEach(func() {
		startStep = make(chan error, 1)
		fakeStepFactoryStep = new(fakes.FakeStepFactory)
		runStep = new(fakes.FakeStep)
		fakeStepFactoryStep.UsingReturns(runStep)

	})

	JustBeforeEach(func() {
		timeout = Timeout(fakeStepFactoryStep, timeoutDuration)
		step = timeout.Using(nil, nil)
		process = ifrit.Background(step)
	})

	Context("when the process goes beyond the duration", func() {
		BeforeEach(func() {
			runStep.ResultStub = successResult(true)
			timeoutDuration = atc.Duration(1 * time.Second)

			runStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				close(ready)
				select {
				case <-startStep:
					return nil
				case <-signals:
					return ErrInterrupted
				}
			}
		})

		It("should interrupt after timeout duration", func() {
			Eventually(runStep.RunCallCount).Should(Equal(1))

			var receivedError error
			Eventually(process.Wait(), 3*time.Second).Should(Receive(&receivedError))
			Ω(receivedError).Should(Equal(ErrStepTimedOut))
		})

		Context("when the process is signaled", func() {
			BeforeEach(func() {
				timeoutDuration = atc.Duration(10 * time.Second)
			})

			It("the process should be interrupted", func() {
				Eventually(runStep.RunCallCount).Should(Equal(1))

				process.Signal(os.Kill)

				var receivedError error
				Eventually(process.Wait()).Should(Receive(&receivedError))
				Ω(receivedError).ShouldNot(BeNil())
				Ω(receivedError.Error()).Should(ContainSubstring(ErrInterrupted.Error()))
			})
		})

		Context("when the step returns an error", func() {
			var someError error

			BeforeEach(func() {
				someError = errors.New("some error")
				runStep.ResultStub = successResult(false)
				runStep.RunReturns(someError)
			})

			It("returns the error", func() {
				var receivedError error
				Eventually(process.Wait()).Should(Receive(&receivedError))
				Ω(receivedError).ShouldNot(BeNil())
				Ω(receivedError).Should(Equal(someError))
			})
		})

		Context("result", func() {
			It("is not successful", func() {
				Eventually(runStep.RunCallCount).Should(Equal(1))

				var receivedError error
				Eventually(process.Wait(), 3*time.Second).Should(Receive(&receivedError))
				Ω(receivedError).ShouldNot(BeNil())

				var success Success
				Ω(step.Result(&success)).Should(BeTrue())
				Ω(bool(success)).Should(BeFalse())
			})
		})
	})

	Context("result", func() {
		Context("when the process does not time out", func() {
			Context("and the step is successful", func() {
				BeforeEach(func() {
					runStep.ResultStub = successResult(true)
				})

				It("is successful", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))

					var success Success
					Ω(step.Result(&success)).Should(BeTrue())
					Ω(bool(success)).Should(BeTrue())
				})
			})

			Context("and the step fails", func() {
				BeforeEach(func() {
					runStep.ResultStub = successResult(false)
				})

				It("is not successful", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))

					var success Success
					Ω(step.Result(&success)).Should(BeTrue())
					Ω(bool(success)).Should(BeFalse())
				})
			})
		})
	})
})
