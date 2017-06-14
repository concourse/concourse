package exec_test

import (
	"errors"
	"os"

	. "github.com/concourse/atc/exec"
	"github.com/tedsuo/ifrit"

	"github.com/concourse/atc/exec/execfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Retry Step", func() {
	var (
		attempt1Factory *execfakes.FakeStepFactory
		attempt1Step    *execfakes.FakeStep

		attempt2Factory *execfakes.FakeStepFactory
		attempt2Step    *execfakes.FakeStep

		attempt3Factory *execfakes.FakeStepFactory
		attempt3Step    *execfakes.FakeStep

		stepFactory StepFactory
		step        Step
	)

	BeforeEach(func() {
		attempt1Factory = new(execfakes.FakeStepFactory)
		attempt1Step = new(execfakes.FakeStep)
		attempt1Factory.UsingReturns(attempt1Step)

		attempt2Factory = new(execfakes.FakeStepFactory)
		attempt2Step = new(execfakes.FakeStep)
		attempt2Factory.UsingReturns(attempt2Step)

		attempt3Factory = new(execfakes.FakeStepFactory)
		attempt3Step = new(execfakes.FakeStep)
		attempt3Factory.UsingReturns(attempt3Step)

		stepFactory = Retry{attempt1Factory, attempt2Factory, attempt3Factory}
		step = stepFactory.Using(nil)
	})

	Context("when attempt 1 succeeds", func() {
		BeforeEach(func() {
			attempt1Step.SucceededReturns(true)
		})

		Describe("Run", func() {
			var process ifrit.Process

			JustBeforeEach(func() {
				process = ifrit.Invoke(step)
			})

			It("returns nil having only run the first attempt", func() {
				Expect(<-process.Wait()).ToNot(HaveOccurred())

				Expect(attempt1Step.RunCallCount()).To(Equal(1))
				Expect(attempt2Step.RunCallCount()).To(Equal(0))
				Expect(attempt3Step.RunCallCount()).To(Equal(0))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 1", func() {
					<-process.Wait()

					// internal check for success within retry loop
					Expect(attempt1Step.SucceededCallCount()).To(Equal(1))

					attempt1Step.SucceededReturns(true)

					Expect(step.Succeeded()).To(BeTrue())

					Expect(attempt1Step.SucceededCallCount()).To(Equal(2))
				})
			})
		})
	})

	Context("when attempt 1 fails, and attempt 2 succeeds", func() {
		BeforeEach(func() {
			attempt1Step.SucceededReturns(false)
			attempt2Step.SucceededReturns(true)
		})

		Describe("Run", func() {
			var process ifrit.Process

			JustBeforeEach(func() {
				process = ifrit.Invoke(step)
			})

			It("returns nil having only run the first and second attempts", func() {
				Expect(<-process.Wait()).ToNot(HaveOccurred())

				Expect(attempt1Step.RunCallCount()).To(Equal(1))
				Expect(attempt2Step.RunCallCount()).To(Equal(1))
				Expect(attempt3Step.RunCallCount()).To(Equal(0))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 2", func() {
					<-process.Wait()

					// internal check for success within retry loop
					Expect(attempt2Step.SucceededCallCount()).To(Equal(1))

					attempt2Step.SucceededReturns(true)

					Expect(step.Succeeded()).To(BeTrue())

					Expect(attempt2Step.SucceededCallCount()).To(Equal(2))
				})
			})
		})
	})

	Context("when attempt 1 errors, and attempt 2 succeeds", func() {
		BeforeEach(func() {
			attempt1Step.RunReturns(errors.New("nope"))
			attempt2Step.SucceededReturns(true)
		})

		Describe("Run", func() {
			var process ifrit.Process

			JustBeforeEach(func() {
				process = ifrit.Invoke(step)
			})

			It("returns nil having only run the first and second attempts", func() {
				Expect(<-process.Wait()).ToNot(HaveOccurred())

				Expect(attempt1Step.RunCallCount()).To(Equal(1))
				Expect(attempt2Step.RunCallCount()).To(Equal(1))
				Expect(attempt3Step.RunCallCount()).To(Equal(0))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 2", func() {
					<-process.Wait()

					// internal check for success within retry loop
					Expect(attempt2Step.SucceededCallCount()).To(Equal(1))

					attempt2Step.SucceededReturns(true)

					Expect(step.Succeeded()).To(BeTrue())

					Expect(attempt2Step.SucceededCallCount()).To(Equal(2))
				})
			})
		})
	})

	Context("when attempt 1 errors, and attempt 2 is interrupted", func() {
		BeforeEach(func() {
			attempt1Step.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				close(ready)
				return errors.New("nope")
			}

			attempt2Step.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				close(ready)
				<-signals
				return ErrInterrupted
			}
		})

		Describe("Run", func() {
			var process ifrit.Process

			JustBeforeEach(func() {
				process = ifrit.Invoke(step)
				process.Signal(os.Interrupt)
			})

			It("returns ErrInterrupted having only run the first and second attempts", func() {
				Expect(<-process.Wait()).To(Equal(ErrInterrupted))

				Expect(attempt1Step.RunCallCount()).To(Equal(1))
				Expect(attempt2Step.RunCallCount()).To(Equal(1))
				Expect(attempt3Step.RunCallCount()).To(Equal(0))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 2", func() {
					<-process.Wait()

					// internal check for success within retry loop
					Expect(attempt2Step.SucceededCallCount()).To(Equal(0))

					attempt2Step.SucceededReturns(true)

					Expect(step.Succeeded()).To(BeTrue())

					Expect(attempt2Step.SucceededCallCount()).To(Equal(1))
				})
			})
		})
	})

	Context("when attempt 1 fails, attempt 2 fails, and attempt 3 succeeds", func() {
		BeforeEach(func() {
			attempt1Step.SucceededReturns(false)
			attempt2Step.SucceededReturns(false)
			attempt3Step.SucceededReturns(true)
		})

		Describe("Run", func() {
			var process ifrit.Process

			JustBeforeEach(func() {
				process = ifrit.Invoke(step)
			})

			It("returns nil having only run the first and second attempts", func() {
				Expect(<-process.Wait()).ToNot(HaveOccurred())

				Expect(attempt1Step.RunCallCount()).To(Equal(1))
				Expect(attempt2Step.RunCallCount()).To(Equal(1))
				Expect(attempt3Step.RunCallCount()).To(Equal(1))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 3", func() {
					<-process.Wait()

					// internal check for success within retry loop
					Expect(attempt3Step.SucceededCallCount()).To(Equal(1))

					attempt3Step.SucceededReturns(true)

					Expect(step.Succeeded()).To(BeTrue())

					Expect(attempt3Step.SucceededCallCount()).To(Equal(2))
				})
			})
		})
	})

	Context("when attempt 1 fails, attempt 2 fails, and attempt 3 errors", func() {
		disaster := errors.New("nope")

		BeforeEach(func() {
			attempt1Step.SucceededReturns(false)
			attempt2Step.SucceededReturns(false)
			attempt3Step.RunReturns(disaster)
		})

		Describe("Run", func() {
			var process ifrit.Process

			JustBeforeEach(func() {
				process = ifrit.Invoke(step)
			})

			It("returns nil having only run the first and second attempts", func() {
				Expect(<-process.Wait()).To(Equal(disaster))

				Expect(attempt1Step.RunCallCount()).To(Equal(1))
				Expect(attempt2Step.RunCallCount()).To(Equal(1))
				Expect(attempt3Step.RunCallCount()).To(Equal(1))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 3", func() {
					<-process.Wait()

					// no internal check for success within retry loop, since it errored
					Expect(attempt3Step.SucceededCallCount()).To(Equal(0))

					attempt3Step.SucceededReturns(true)

					Expect(step.Succeeded()).To(BeTrue())

					Expect(attempt3Step.SucceededCallCount()).To(Equal(1))
				})
			})
		})
	})

	Context("when attempt 1 fails, attempt 2 fails, and attempt 3 fails", func() {
		BeforeEach(func() {
			attempt1Step.SucceededReturns(false)
			attempt2Step.SucceededReturns(false)
			attempt3Step.SucceededReturns(true)
		})

		Describe("Run", func() {
			var process ifrit.Process

			JustBeforeEach(func() {
				process = ifrit.Invoke(step)
			})

			It("returns nil having only run the first and second attempts", func() {
				Expect(<-process.Wait()).ToNot(HaveOccurred())

				Expect(attempt1Step.RunCallCount()).To(Equal(1))
				Expect(attempt2Step.RunCallCount()).To(Equal(1))
				Expect(attempt3Step.RunCallCount()).To(Equal(1))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 3", func() {
					<-process.Wait()

					// internal check for success within retry loop
					Expect(attempt3Step.SucceededCallCount()).To(Equal(1))

					attempt3Step.SucceededReturns(true)

					Expect(step.Succeeded()).To(BeTrue())

					Expect(attempt3Step.SucceededCallCount()).To(Equal(2))
				})
			})
		})
	})
})
