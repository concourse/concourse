package exec_test

import (
	"errors"
	"os"

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

		timeoutDuration string
	)

	BeforeEach(func() {
		startStep = make(chan error, 1)
		fakeStepFactoryStep = new(fakes.FakeStepFactory)
		runStep = new(fakes.FakeStep)
		fakeStepFactoryStep.UsingReturns(runStep)

	})

	Context("when the process is invoked with invoke", func() {
		It("exits successfully", func() {
			timeout = Timeout(fakeStepFactoryStep, timeoutDuration)
			step = timeout.Using(nil, nil)
			process = ifrit.Invoke(step)

			Eventually(process.Ready()).Should(BeClosed())
		})
	})

	Context("when we pass an invalid duration", func() {
		It("errors", func() {
			timeout = Timeout(fakeStepFactoryStep, "nope")
			step = timeout.Using(nil, nil)
			ready := make(chan struct{})
			err := step.Run(nil, ready)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the process is invoked with background", func() {
		JustBeforeEach(func() {
			timeout = Timeout(fakeStepFactoryStep, timeoutDuration)
			step = timeout.Using(nil, nil)
			process = ifrit.Background(step)
		})

		Context("when the process goes beyond the duration", func() {
			BeforeEach(func() {
				runStep.ResultStub = successResult(true)
				timeoutDuration = "1s"

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
				Expect(<-process.Wait()).ToNot(HaveOccurred())
			})

			Context("when the process is signaled", func() {
				BeforeEach(func() {
					timeoutDuration = "10s"
				})

				It("the process should be interrupted", func() {
					Eventually(runStep.RunCallCount).Should(Equal(1))

					process.Signal(os.Kill)

					var receivedError error
					Eventually(process.Wait()).Should(Receive(&receivedError))
					Expect(receivedError).NotTo(BeNil())
					Expect(receivedError.Error()).To(ContainSubstring(ErrInterrupted.Error()))
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
					Expect(receivedError).NotTo(BeNil())
					Expect(receivedError).To(Equal(someError))
				})
			})

			Context("result", func() {
				It("is not successful", func() {
					Eventually(runStep.RunCallCount).Should(Equal(1))

					Expect(<-process.Wait()).To(Succeed())

					var success Success
					Expect(step.Result(&success)).To(BeTrue())
					Expect(bool(success)).To(BeFalse())
				})
			})
		})

		Context("result", func() {
			Context("when the process does not time out", func() {
				BeforeEach(func() {
					timeoutDuration = "10s"
				})

				Context("and the step is successful", func() {
					BeforeEach(func() {
						runStep.ResultStub = successResult(true)
					})

					It("is successful", func() {
						Eventually(process.Wait()).Should(Receive(BeNil()))

						var success Success
						Expect(step.Result(&success)).To(BeTrue())
						Expect(bool(success)).To(BeTrue())
					})
				})

				Context("and the step fails", func() {
					BeforeEach(func() {
						runStep.ResultStub = successResult(false)
					})

					It("is not successful", func() {
						Eventually(process.Wait()).Should(Receive(BeNil()))

						var success Success
						Expect(step.Result(&success)).To(BeTrue())
						Expect(bool(success)).To(BeFalse())
					})
				})
			})
		})
	})
})
