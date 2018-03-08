package exec_test

import (
	"context"
	"errors"

	. "github.com/concourse/atc/exec"

	"github.com/concourse/atc/exec/execfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Retry Step", func() {
	var (
		ctx    context.Context
		cancel func()

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
		ctx, cancel = context.WithCancel(context.Background())

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
			var stepErr error

			JustBeforeEach(func() {
				stepErr = step.Run(ctx)
			})

			It("returns nil having only run the first attempt", func() {
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(attempt1Step.RunCallCount()).To(Equal(1))
				Expect(attempt2Step.RunCallCount()).To(Equal(0))
				Expect(attempt3Step.RunCallCount()).To(Equal(0))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 1", func() {
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
			var stepErr error

			JustBeforeEach(func() {
				stepErr = step.Run(ctx)
			})

			It("returns nil having only run the first and second attempts", func() {
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(attempt1Step.RunCallCount()).To(Equal(1))
				Expect(attempt2Step.RunCallCount()).To(Equal(1))
				Expect(attempt3Step.RunCallCount()).To(Equal(0))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 2", func() {
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
			var stepErr error

			JustBeforeEach(func() {
				stepErr = step.Run(ctx)
			})

			It("returns nil having only run the first and second attempts", func() {
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(attempt1Step.RunCallCount()).To(Equal(1))
				Expect(attempt2Step.RunCallCount()).To(Equal(1))
				Expect(attempt3Step.RunCallCount()).To(Equal(0))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 2", func() {
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
			attempt1Step.RunReturns(errors.New("nope"))
			attempt2Step.RunStub = func(c context.Context) error {
				cancel()
				return c.Err()
			}
		})

		Describe("Run", func() {
			var stepErr error

			JustBeforeEach(func() {
				stepErr = step.Run(ctx)
			})

			It("returns the context error having only run the first and second attempts", func() {
				Expect(stepErr).To(Equal(context.Canceled))

				Expect(attempt1Step.RunCallCount()).To(Equal(1))
				Expect(attempt2Step.RunCallCount()).To(Equal(1))
				Expect(attempt3Step.RunCallCount()).To(Equal(0))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 2", func() {
					// internal check for success within retry loop
					Expect(attempt2Step.SucceededCallCount()).To(Equal(0))

					attempt2Step.SucceededReturns(true)

					Expect(step.Succeeded()).To(BeTrue())

					Expect(attempt2Step.SucceededCallCount()).To(Equal(1))
				})
			})
		})
	})

	Context("when attempt 1 errors, attempt 2 times out, and attempt 3 succeeds", func() {
		BeforeEach(func() {
			attempt1Step.RunReturns(errors.New("nope"))
			attempt2Step.RunStub = func(c context.Context) error {
				timeout, subCancel := context.WithTimeout(c, 0)
				defer subCancel()
				<-timeout.Done()
				return timeout.Err()
			}
		})

		Describe("Run", func() {
			var stepErr error

			JustBeforeEach(func() {
				stepErr = step.Run(ctx)
			})

			It("returns nil after running all 3 steps", func() {
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(attempt1Step.RunCallCount()).To(Equal(1))
				Expect(attempt2Step.RunCallCount()).To(Equal(1))
				Expect(attempt3Step.RunCallCount()).To(Equal(1))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 3", func() {
					// internal check for success within retry loop
					Expect(attempt3Step.SucceededCallCount()).To(Equal(1))

					attempt3Step.SucceededReturns(true)

					Expect(step.Succeeded()).To(BeTrue())

					Expect(attempt3Step.SucceededCallCount()).To(Equal(2))
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
			var stepErr error

			JustBeforeEach(func() {
				stepErr = step.Run(ctx)
			})

			It("returns nil after running all 3 steps", func() {
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(attempt1Step.RunCallCount()).To(Equal(1))
				Expect(attempt2Step.RunCallCount()).To(Equal(1))
				Expect(attempt3Step.RunCallCount()).To(Equal(1))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 3", func() {
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
			var stepErr error

			JustBeforeEach(func() {
				stepErr = step.Run(ctx)
			})

			It("returns the error", func() {
				Expect(stepErr).To(Equal(disaster))

				Expect(attempt1Step.RunCallCount()).To(Equal(1))
				Expect(attempt2Step.RunCallCount()).To(Equal(1))
				Expect(attempt3Step.RunCallCount()).To(Equal(1))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 3", func() {
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
			var stepErr error

			JustBeforeEach(func() {
				stepErr = step.Run(ctx)
			})

			It("returns nil having only run the first and second attempts", func() {
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(attempt1Step.RunCallCount()).To(Equal(1))
				Expect(attempt2Step.RunCallCount()).To(Equal(1))
				Expect(attempt3Step.RunCallCount()).To(Equal(1))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 3", func() {
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
