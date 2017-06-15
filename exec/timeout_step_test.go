package exec_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	. "github.com/concourse/atc/exec"

	"github.com/concourse/atc/exec/execfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/tedsuo/ifrit"
)

var _ = Describe("Timeout Step", func() {
	var (
		fakeStepFactoryStep *execfakes.FakeStepFactory

		runStep *execfakes.FakeStep

		timeout StepFactory
		step    Step

		process ifrit.Process

		timeoutDuration string
		fakeClock       *fakeclock.FakeClock
	)

	BeforeEach(func() {
		fakeStepFactoryStep = new(execfakes.FakeStepFactory)
		runStep = new(execfakes.FakeStep)
		fakeStepFactoryStep.UsingReturns(runStep)

		timeoutDuration = "1h"
		fakeClock = fakeclock.NewFakeClock(time.Now())
	})

	JustBeforeEach(func() {
		timeout = Timeout(fakeStepFactoryStep, timeoutDuration, fakeClock)
		step = timeout.Using(nil)
		process = ifrit.Background(step)
	})

	Context("when the duration is invalid", func() {
		BeforeEach(func() {
			timeoutDuration = "nope"
		})

		It("errors immediately", func() {
			Expect(<-process.Wait()).To(HaveOccurred())
			Expect(process.Ready()).ToNot(BeClosed())
		})
	})

	Context("when the process goes beyond the duration", func() {
		var receivedSignals <-chan os.Signal

		BeforeEach(func() {
			s := make(chan os.Signal, 1)
			receivedSignals = s

			runStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				close(ready)
				fakeClock.Increment(time.Hour)
				s <- <-signals
				return nil
			}
		})

		It("interrupts it", func() {
			<-process.Wait()

			Expect(receivedSignals).To(Receive(Equal(os.Interrupt)))
		})

		It("exits with no error", func() {
			Expect(<-process.Wait()).ToNot(HaveOccurred())
		})

		Describe("result", func() {
			It("is not successful", func() {
				Eventually(runStep.RunCallCount).Should(Equal(1))

				Expect(<-process.Wait()).To(Succeed())

				Expect(step.Succeeded()).To(BeFalse())
			})
		})
	})

	Context("when the step returns an error", func() {
		var someError error

		BeforeEach(func() {
			someError = errors.New("some error")
			runStep.SucceededReturns(false)
			runStep.RunReturns(someError)
		})

		It("returns the error", func() {
			var receivedError error
			Eventually(process.Wait()).Should(Receive(&receivedError))
			Expect(receivedError).NotTo(BeNil())
			Expect(receivedError).To(Equal(someError))
		})
	})

	Context("when the step completes within the duration", func() {
		BeforeEach(func() {
			runStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				close(ready)
				fakeClock.Increment(time.Hour / 2)
				return nil
			}
		})

		It("does not interrupt it", func() {
			<-process.Wait()

			Expect(runStep.RunCallCount()).To(Equal(1))

			subSignals, _ := runStep.RunArgsForCall(0)
			Expect(subSignals).ToNot(Receive())
		})

		It("exits with no error", func() {
			Expect(<-process.Wait()).ToNot(HaveOccurred())
		})

		Context("when the step is successful", func() {
			BeforeEach(func() {
				runStep.SucceededReturns(true)
			})

			It("is successful", func() {
				Eventually(process.Wait()).Should(Receive(BeNil()))

				Expect(step.Succeeded()).To(BeTrue())
			})
		})

		Context("when the step fails", func() {
			BeforeEach(func() {
				runStep.SucceededReturns(false)
			})

			It("is not successful", func() {
				Eventually(process.Wait()).Should(Receive(BeNil()))

				Expect(step.Succeeded()).To(BeFalse())
			})
		})

		Describe("signalling", func() {
			var receivedSignals <-chan os.Signal

			BeforeEach(func() {
				s := make(chan os.Signal, 1)
				receivedSignals = s

				runStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
					close(ready)
					fakeClock.Increment(time.Hour / 2)
					s <- <-signals
					return nil
				}
			})

			It("forwards the signal down", func() {
				process.Signal(os.Kill)

				Expect(<-process.Wait()).ToNot(HaveOccurred())
				Expect(<-receivedSignals).To(Equal(os.Kill))
			})
		})
	})
})
