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

var _ = Describe("On Failure Step", func() {
	var (
		ctx    context.Context
		cancel func()

		step *execfakes.FakeStep
		hook *execfakes.FakeStep

		repo  *build.Repository
		state *execfakes.FakeRunState

		onFailureStep exec.Step

		stepOk  bool
		stepErr error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		step = &execfakes.FakeStep{}
		hook = &execfakes.FakeStep{}

		repo = build.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactRepositoryReturns(repo)

		onFailureStep = exec.OnFailure(step, hook)
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		stepOk, stepErr = onFailureStep.Run(ctx, state)
	})

	Context("when the step fails", func() {
		BeforeEach(func() {
			step.RunReturns(false, nil)
		})

		It("runs the failure hook", func() {
			Expect(step.RunCallCount()).To(Equal(1))
			Expect(hook.RunCallCount()).To(Equal(1))
		})

		It("runs the hook with the run state", func() {
			Expect(hook.RunCallCount()).To(Equal(1))

			_, argsState := hook.RunArgsForCall(0)
			Expect(argsState).To(Equal(state))
		})

		It("propagates the context to the hook", func() {
			runCtx, _ := hook.RunArgsForCall(0)
			Expect(runCtx).To(Equal(ctx))
		})

		It("does not error", func() {
			Expect(stepErr).ToNot(HaveOccurred())
		})
	})

	Context("when the step errors", func() {
		disaster := errors.New("disaster")

		BeforeEach(func() {
			step.RunReturns(false, disaster)
		})

		It("does not run the failure hook", func() {
			Expect(step.RunCallCount()).To(Equal(1))
			Expect(hook.RunCallCount()).To(Equal(0))
		})

		It("returns the error", func() {
			Expect(stepErr).To(Equal(disaster))
		})
	})

	Context("when the step succeeds", func() {
		BeforeEach(func() {
			step.RunReturns(true, nil)
		})

		It("does not run the failure hook", func() {
			Expect(step.RunCallCount()).To(Equal(1))
			Expect(hook.RunCallCount()).To(Equal(0))
		})

		It("returns nil", func() {
			Expect(stepErr).To(BeNil())
		})
	})

	It("propagates the context to the step", func() {
		runCtx, _ := step.RunArgsForCall(0)
		Expect(runCtx).To(Equal(ctx))
	})

	Context("when step fails and hook fails", func() {
		BeforeEach(func() {
			step.RunReturns(false, nil)
			hook.RunReturns(false, nil)
		})

		It("fails", func() {
			Expect(stepOk).To(BeFalse())
		})
	})

	Context("when step fails and hook succeeds", func() {
		BeforeEach(func() {
			step.RunReturns(false, nil)
			hook.RunReturns(true, nil)
		})

		It("fails", func() {
			Expect(stepOk).To(BeFalse())
		})
	})

	Context("when step succeeds", func() {
		BeforeEach(func() {
			step.RunReturns(true, nil)
		})

		It("succeeds", func() {
			Expect(stepOk).To(BeTrue())
		})
	})
})
