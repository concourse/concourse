package exec_test

import (
	"context"
	"errors"

	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Ensure Step", func() {
	var (
		ctx    context.Context
		cancel func()

		step *execfakes.FakeStep
		hook *execfakes.FakeStep

		repo  *build.Repository
		state *execfakes.FakeRunState

		ensure exec.Step

		stepOk  bool
		stepErr error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		step = &execfakes.FakeStep{}
		hook = &execfakes.FakeStep{}

		step = &execfakes.FakeStep{}
		hook = &execfakes.FakeStep{}

		step.RunStub = func(ctx context.Context, state exec.RunState) (bool, error) {
			return true, ctx.Err()
		}

		hook.RunStub = func(ctx context.Context, state exec.RunState) (bool, error) {
			return true, ctx.Err()
		}

		repo = build.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactRepositoryReturns(repo)

		ensure = exec.Ensure(step, hook)
	})

	JustBeforeEach(func() {
		stepOk, stepErr = ensure.Run(ctx, state)
	})

	Context("when the step succeeds", func() {
		BeforeEach(func() {
			step.RunReturns(true, nil)
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
			step.RunReturns(false, nil)
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
			step.RunReturns(false, disaster)
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
			hook.RunStub = func(context.Context, exec.RunState) (bool, error) {
				cancel()
				return false, ctx.Err()
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

	Context("when both step and hook succeed", func() {
		BeforeEach(func() {
			step.RunReturns(true, nil)
			hook.RunReturns(true, nil)
		})

		It("succeeds", func() {
			Expect(stepOk).To(BeTrue())
		})
	})

	Context("when step succeeds and hook fails", func() {
		BeforeEach(func() {
			step.RunReturns(true, nil)
			hook.RunReturns(false, nil)
		})

		It("does not succeed", func() {
			Expect(stepOk).To(BeFalse())
		})
	})

	Context("when step fails and hook succeeds", func() {
		BeforeEach(func() {
			step.RunReturns(false, nil)
			hook.RunReturns(true, nil)
		})

		It("does not succeed", func() {
			Expect(stepOk).To(BeFalse())
		})
	})

	Context("when step succeeds and hook fails", func() {
		BeforeEach(func() {
			step.RunReturns(false, nil)
			hook.RunReturns(false, nil)
		})

		It("does not succeed", func() {
			Expect(stepOk).To(BeFalse())
		})
	})
})
