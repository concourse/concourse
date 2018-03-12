package exec_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/execfakes"
	"github.com/concourse/atc/worker"
)

var _ = Describe("Ensure Step", func() {
	var (
		ctx    context.Context
		cancel func()

		step *execfakes.FakeStep
		hook *execfakes.FakeStep

		repo  *worker.ArtifactRepository
		state *execfakes.FakeRunState

		ensure exec.Step

		stepErr error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		step = &execfakes.FakeStep{}
		hook = &execfakes.FakeStep{}

		step = &execfakes.FakeStep{}
		hook = &execfakes.FakeStep{}

		step.RunStub = func(ctx context.Context, state exec.RunState) error {
			return ctx.Err()
		}

		hook.RunStub = func(ctx context.Context, state exec.RunState) error {
			return ctx.Err()
		}

		repo = worker.NewArtifactRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactsReturns(repo)

		ensure = exec.Ensure(step, hook)
	})

	JustBeforeEach(func() {
		stepErr = ensure.Run(ctx, state)
	})

	Context("when the step succeeds", func() {
		BeforeEach(func() {
			step.SucceededReturns(true)
		})

		It("returns nil", func() {
			Expect(stepErr).ToNot(HaveOccurred())
		})

		It("runs the ensure hook", func() {
			Expect(step.RunCallCount()).To(Equal(1))
			Expect(hook.RunCallCount()).To(Equal(1))
		})
	})

	Context("when the step fails", func() {
		BeforeEach(func() {
			step.SucceededReturns(false)
		})

		It("returns nil", func() {
			Expect(stepErr).ToNot(HaveOccurred())
		})

		It("runs the ensure hook", func() {
			Expect(step.RunCallCount()).To(Equal(1))
			Expect(hook.RunCallCount()).To(Equal(1))
		})
	})

	Context("when the step errors", func() {
		disaster := errors.New("disaster")

		BeforeEach(func() {
			step.RunReturns(disaster)
		})

		It("returns the error", func() {
			Expect(stepErr).To(HaveOccurred())
			Expect(stepErr.Error()).To(ContainSubstring("disaster"))
		})

		It("runs the ensure hook", func() {
			Expect(step.RunCallCount()).To(Equal(1))
			Expect(hook.RunCallCount()).To(Equal(1))
		})
	})

	Context("when the context is canceled during the first step", func() {
		BeforeEach(func() {
			cancel()
		})

		It("returns context.Canceled", func() {
			Expect(stepErr).To(Equal(context.Canceled))
		})

		It("cancels the first step and runs the hook (without canceling it)", func() {
			Expect(step.RunCallCount()).To(Equal(1))
			Expect(hook.RunCallCount()).To(Equal(1))

			stepCtx, _ := step.RunArgsForCall(0)
			Expect(stepCtx.Err()).To(Equal(context.Canceled))

			hookCtx, _ := hook.RunArgsForCall(0)
			Expect(hookCtx.Err()).ToNot(HaveOccurred())
		})
	})

	Context("when the context is canceled during the hook", func() {
		BeforeEach(func() {
			hook.RunStub = func(context.Context, exec.RunState) error {
				cancel()
				return ctx.Err()
			}
		})

		It("returns context.Canceled", func() {
			Expect(stepErr).To(Equal(context.Canceled))
		})

		It("allows canceling the hook if the first step has not been canceled", func() {
			Expect(step.RunCallCount()).To(Equal(1))
			Expect(hook.RunCallCount()).To(Equal(1))

			stepCtx, _ := step.RunArgsForCall(0)
			Expect(stepCtx.Err()).To(Equal(context.Canceled))

			hookCtx, _ := hook.RunArgsForCall(0)
			Expect(hookCtx.Err()).To(Equal(context.Canceled))
		})
	})

	Describe("Succeeded", func() {
		Context("when the provided interface is type Success", func() {
			Context("when both step and hook succeed", func() {
				BeforeEach(func() {
					step.SucceededReturns(true)
					hook.SucceededReturns(true)
				})

				It("succeeds", func() {
					Expect(ensure.Succeeded()).To(BeTrue())
				})
			})

			Context("when step succeeds and hook fails", func() {
				BeforeEach(func() {
					step.SucceededReturns(true)
					hook.SucceededReturns(false)
				})

				It("does not succeed", func() {
					Expect(ensure.Succeeded()).To(BeFalse())
				})
			})

			Context("when step fails and hook succeeds", func() {
				BeforeEach(func() {
					step.SucceededReturns(false)
					hook.SucceededReturns(true)
				})

				It("does not succeed", func() {
					Expect(ensure.Succeeded()).To(BeFalse())
				})
			})

			Context("when step succeeds and hook fails", func() {
				BeforeEach(func() {
					step.SucceededReturns(false)
					hook.SucceededReturns(false)
				})

				It("does not succeed", func() {
					Expect(ensure.Succeeded()).To(BeFalse())
				})
			})
		})
	})
})
