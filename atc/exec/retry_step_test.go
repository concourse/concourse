package exec_test

import (
	"context"
	"errors"

	. "github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Retry Step", func() {
	var (
		ctx    context.Context
		cancel func()

		attempt1 *execfakes.FakeStep
		attempt2 *execfakes.FakeStep
		attempt3 *execfakes.FakeStep

		repo  *build.Repository
		state *execfakes.FakeRunState

		step Step
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		attempt1 = new(execfakes.FakeStep)
		attempt2 = new(execfakes.FakeStep)
		attempt3 = new(execfakes.FakeStep)

		repo = build.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactRepositoryReturns(repo)

		step = Retry(attempt1, attempt2, attempt3)
	})

	Describe("Run", func() {
		var stepOk bool
		var stepErr error

		JustBeforeEach(func() {
			stepOk, stepErr = step.Run(ctx, state)
		})

		Context("when attempt 1 succeeds", func() {
			BeforeEach(func() {
				attempt1.RunReturns(true, nil)
			})

			It("returns nil having only run the first attempt", func() {
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(attempt1.RunCallCount()).To(Equal(1))
				Expect(attempt2.RunCallCount()).To(Equal(0))
				Expect(attempt3.RunCallCount()).To(Equal(0))
			})

			It("succeeds", func() {
				Expect(stepOk).To(BeTrue())
			})
		})

		Context("when attempt 1 fails, and attempt 2 succeeds", func() {
			BeforeEach(func() {
				attempt1.RunReturns(false, nil)
				attempt2.RunReturns(true, nil)
			})

			It("returns nil having only run the first and second attempts", func() {
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(attempt1.RunCallCount()).To(Equal(1))
				Expect(attempt2.RunCallCount()).To(Equal(1))
				Expect(attempt3.RunCallCount()).To(Equal(0))
			})

			It("succeeds", func() {
				Expect(stepOk).To(BeTrue())
			})
		})

		Context("when attempt 1 errors, and attempt 2 succeeds", func() {
			BeforeEach(func() {
				attempt1.RunReturns(false, errors.New("nope"))
				attempt2.RunReturns(true, nil)
			})

			It("returns nil having only run the first and second attempts", func() {
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(attempt1.RunCallCount()).To(Equal(1))
				Expect(attempt2.RunCallCount()).To(Equal(1))
				Expect(attempt3.RunCallCount()).To(Equal(0))
			})

			It("succeeds", func() {
				Expect(stepOk).To(BeTrue())
			})
		})

		Context("when attempt 1 errors, and attempt 2 is interrupted", func() {
			BeforeEach(func() {
				attempt1.RunReturns(false, errors.New("nope"))
				attempt2.RunStub = func(c context.Context, r RunState) (bool, error) {
					cancel()
					return false, c.Err()
				}
			})

			It("returns the context error having only run the first and second attempts", func() {
				Expect(stepErr).To(Equal(context.Canceled))

				Expect(attempt1.RunCallCount()).To(Equal(1))
				Expect(attempt2.RunCallCount()).To(Equal(1))
				Expect(attempt3.RunCallCount()).To(Equal(0))
			})

			It("fails", func() {
				Expect(stepOk).To(BeFalse())
			})
		})

		Context("when attempt 1 errors, attempt 2 times out, and attempt 3 succeeds", func() {
			BeforeEach(func() {
				attempt1.RunReturns(false, errors.New("nope"))
				attempt2.RunStub = func(c context.Context, r RunState) (bool, error) {
					timeout, subCancel := context.WithTimeout(c, 0)
					defer subCancel()
					<-timeout.Done()
					return false, timeout.Err()
				}
				attempt3.RunReturns(true, nil)
			})

			It("returns nil after running all 3 steps", func() {
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(attempt1.RunCallCount()).To(Equal(1))
				Expect(attempt2.RunCallCount()).To(Equal(1))
				Expect(attempt3.RunCallCount()).To(Equal(1))
			})

			It("succeeds", func() {
				Expect(stepOk).To(BeTrue())
			})
		})

		Context("when attempt 1 fails, attempt 2 fails, and attempt 3 succeeds", func() {
			BeforeEach(func() {
				attempt1.RunReturns(false, nil)
				attempt2.RunReturns(false, nil)
				attempt3.RunReturns(true, nil)
			})

			It("returns nil after running all 3 steps", func() {
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(attempt1.RunCallCount()).To(Equal(1))
				Expect(attempt2.RunCallCount()).To(Equal(1))
				Expect(attempt3.RunCallCount()).To(Equal(1))
			})

			It("succeeds", func() {
				Expect(stepOk).To(BeTrue())
			})
		})

		Context("when attempt 1 fails, attempt 2 fails, and attempt 3 errors", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				attempt1.RunReturns(false, nil)
				attempt2.RunReturns(false, nil)
				attempt3.RunReturns(false, disaster)
			})

			It("returns the error", func() {
				Expect(stepErr).To(Equal(disaster))

				Expect(attempt1.RunCallCount()).To(Equal(1))
				Expect(attempt2.RunCallCount()).To(Equal(1))
				Expect(attempt3.RunCallCount()).To(Equal(1))
			})

			It("fails", func() {
				Expect(stepOk).To(BeFalse())
			})
		})

		Context("when attempt 1 fails, attempt 2 fails, and attempt 3 fails", func() {
			BeforeEach(func() {
				attempt1.RunReturns(false, nil)
				attempt2.RunReturns(false, nil)
				attempt3.RunReturns(false, nil)
			})

			It("returns nil having only run the first and second attempts", func() {
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(attempt1.RunCallCount()).To(Equal(1))
				Expect(attempt2.RunCallCount()).To(Equal(1))
				Expect(attempt3.RunCallCount()).To(Equal(1))
			})

			It("fails", func() {
				Expect(stepOk).To(BeFalse())
			})
		})
	})
})
