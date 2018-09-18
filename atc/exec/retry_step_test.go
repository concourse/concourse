package exec_test

import (
	"context"
	"errors"

	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/worker"

	"github.com/concourse/atc/exec/execfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Retry Step", func() {
	var (
		ctx    context.Context
		cancel func()

		attempt1 *execfakes.FakeStep
		attempt2 *execfakes.FakeStep
		attempt3 *execfakes.FakeStep

		repo  *worker.ArtifactRepository
		state *execfakes.FakeRunState

		step Step
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		attempt1 = new(execfakes.FakeStep)
		attempt2 = new(execfakes.FakeStep)
		attempt3 = new(execfakes.FakeStep)

		repo = worker.NewArtifactRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactsReturns(repo)

		step = Retry(attempt1, attempt2, attempt3)
	})

	Context("when attempt 1 succeeds", func() {
		BeforeEach(func() {
			attempt1.SucceededReturns(true)
		})

		Describe("Run", func() {
			var stepErr error

			JustBeforeEach(func() {
				stepErr = step.Run(ctx, state)
			})

			It("returns nil having only run the first attempt", func() {
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(attempt1.RunCallCount()).To(Equal(1))
				Expect(attempt2.RunCallCount()).To(Equal(0))
				Expect(attempt3.RunCallCount()).To(Equal(0))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 1", func() {
					// internal check for success within retry loop
					Expect(attempt1.SucceededCallCount()).To(Equal(1))

					attempt1.SucceededReturns(true)

					Expect(step.Succeeded()).To(BeTrue())

					Expect(attempt1.SucceededCallCount()).To(Equal(2))
				})
			})
		})
	})

	Context("when attempt 1 fails, and attempt 2 succeeds", func() {
		BeforeEach(func() {
			attempt1.SucceededReturns(false)
			attempt2.SucceededReturns(true)
		})

		Describe("Run", func() {
			var stepErr error

			JustBeforeEach(func() {
				stepErr = step.Run(ctx, state)
			})

			It("returns nil having only run the first and second attempts", func() {
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(attempt1.RunCallCount()).To(Equal(1))
				Expect(attempt2.RunCallCount()).To(Equal(1))
				Expect(attempt3.RunCallCount()).To(Equal(0))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 2", func() {
					// internal check for success within retry loop
					Expect(attempt2.SucceededCallCount()).To(Equal(1))

					attempt2.SucceededReturns(true)

					Expect(step.Succeeded()).To(BeTrue())

					Expect(attempt2.SucceededCallCount()).To(Equal(2))
				})
			})
		})
	})

	Context("when attempt 1 errors, and attempt 2 succeeds", func() {
		BeforeEach(func() {
			attempt1.RunReturns(errors.New("nope"))
			attempt2.SucceededReturns(true)
		})

		Describe("Run", func() {
			var stepErr error

			JustBeforeEach(func() {
				stepErr = step.Run(ctx, state)
			})

			It("returns nil having only run the first and second attempts", func() {
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(attempt1.RunCallCount()).To(Equal(1))
				Expect(attempt2.RunCallCount()).To(Equal(1))
				Expect(attempt3.RunCallCount()).To(Equal(0))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 2", func() {
					// internal check for success within retry loop
					Expect(attempt2.SucceededCallCount()).To(Equal(1))

					attempt2.SucceededReturns(true)

					Expect(step.Succeeded()).To(BeTrue())

					Expect(attempt2.SucceededCallCount()).To(Equal(2))
				})
			})
		})
	})

	Context("when attempt 1 errors, and attempt 2 is interrupted", func() {
		BeforeEach(func() {
			attempt1.RunReturns(errors.New("nope"))
			attempt2.RunStub = func(c context.Context, r RunState) error {
				cancel()
				return c.Err()
			}
		})

		Describe("Run", func() {
			var stepErr error

			JustBeforeEach(func() {
				stepErr = step.Run(ctx, state)
			})

			It("returns the context error having only run the first and second attempts", func() {
				Expect(stepErr).To(Equal(context.Canceled))

				Expect(attempt1.RunCallCount()).To(Equal(1))
				Expect(attempt2.RunCallCount()).To(Equal(1))
				Expect(attempt3.RunCallCount()).To(Equal(0))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 2", func() {
					// internal check for success within retry loop
					Expect(attempt2.SucceededCallCount()).To(Equal(0))

					attempt2.SucceededReturns(true)

					Expect(step.Succeeded()).To(BeTrue())

					Expect(attempt2.SucceededCallCount()).To(Equal(1))
				})
			})
		})
	})

	Context("when attempt 1 errors, attempt 2 times out, and attempt 3 succeeds", func() {
		BeforeEach(func() {
			attempt1.RunReturns(errors.New("nope"))
			attempt2.RunStub = func(c context.Context, r RunState) error {
				timeout, subCancel := context.WithTimeout(c, 0)
				defer subCancel()
				<-timeout.Done()
				return timeout.Err()
			}
		})

		Describe("Run", func() {
			var stepErr error

			JustBeforeEach(func() {
				stepErr = step.Run(ctx, state)
			})

			It("returns nil after running all 3 steps", func() {
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(attempt1.RunCallCount()).To(Equal(1))
				Expect(attempt2.RunCallCount()).To(Equal(1))
				Expect(attempt3.RunCallCount()).To(Equal(1))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 3", func() {
					// internal check for success within retry loop
					Expect(attempt3.SucceededCallCount()).To(Equal(1))

					attempt3.SucceededReturns(true)

					Expect(step.Succeeded()).To(BeTrue())

					Expect(attempt3.SucceededCallCount()).To(Equal(2))
				})
			})
		})
	})

	Context("when attempt 1 fails, attempt 2 fails, and attempt 3 succeeds", func() {
		BeforeEach(func() {
			attempt1.SucceededReturns(false)
			attempt2.SucceededReturns(false)
			attempt3.SucceededReturns(true)
		})

		Describe("Run", func() {
			var stepErr error

			JustBeforeEach(func() {
				stepErr = step.Run(ctx, state)
			})

			It("returns nil after running all 3 steps", func() {
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(attempt1.RunCallCount()).To(Equal(1))
				Expect(attempt2.RunCallCount()).To(Equal(1))
				Expect(attempt3.RunCallCount()).To(Equal(1))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 3", func() {
					// internal check for success within retry loop
					Expect(attempt3.SucceededCallCount()).To(Equal(1))

					attempt3.SucceededReturns(true)

					Expect(step.Succeeded()).To(BeTrue())

					Expect(attempt3.SucceededCallCount()).To(Equal(2))
				})
			})
		})
	})

	Context("when attempt 1 fails, attempt 2 fails, and attempt 3 errors", func() {
		disaster := errors.New("nope")

		BeforeEach(func() {
			attempt1.SucceededReturns(false)
			attempt2.SucceededReturns(false)
			attempt3.RunReturns(disaster)
		})

		Describe("Run", func() {
			var stepErr error

			JustBeforeEach(func() {
				stepErr = step.Run(ctx, state)
			})

			It("returns the error", func() {
				Expect(stepErr).To(Equal(disaster))

				Expect(attempt1.RunCallCount()).To(Equal(1))
				Expect(attempt2.RunCallCount()).To(Equal(1))
				Expect(attempt3.RunCallCount()).To(Equal(1))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 3", func() {
					// no internal check for success within retry loop, since it errored
					Expect(attempt3.SucceededCallCount()).To(Equal(0))

					attempt3.SucceededReturns(true)

					Expect(step.Succeeded()).To(BeTrue())

					Expect(attempt3.SucceededCallCount()).To(Equal(1))
				})
			})
		})
	})

	Context("when attempt 1 fails, attempt 2 fails, and attempt 3 fails", func() {
		BeforeEach(func() {
			attempt1.SucceededReturns(false)
			attempt2.SucceededReturns(false)
			attempt3.SucceededReturns(true)
		})

		Describe("Run", func() {
			var stepErr error

			JustBeforeEach(func() {
				stepErr = step.Run(ctx, state)
			})

			It("returns nil having only run the first and second attempts", func() {
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(attempt1.RunCallCount()).To(Equal(1))
				Expect(attempt2.RunCallCount()).To(Equal(1))
				Expect(attempt3.RunCallCount()).To(Equal(1))
			})

			Describe("Succeeded", func() {
				It("delegates to attempt 3", func() {
					// internal check for success within retry loop
					Expect(attempt3.SucceededCallCount()).To(Equal(1))

					attempt3.SucceededReturns(true)

					Expect(step.Succeeded()).To(BeTrue())

					Expect(attempt3.SucceededCallCount()).To(Equal(2))
				})
			})
		})
	})
})
