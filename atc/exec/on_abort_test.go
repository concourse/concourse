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

var _ = Describe("On Abort Step", func() {
	var (
		ctx    context.Context
		cancel func()

		step *execfakes.FakeStep
		hook *execfakes.FakeStep

		repo  *build.Repository
		state *execfakes.FakeRunState

		onAbortStep exec.Step

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

		onAbortStep = exec.OnAbort(step, hook)

		stepOk = false
		stepErr = nil
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		stepOk, stepErr = onAbortStep.Run(ctx, state)
	})

	Context("when the step is aborted", func() {
		BeforeEach(func() {
			step.RunReturns(false, context.Canceled)
		})

		It("runs the abort hook", func() {
			Expect(stepErr).To(Equal(context.Canceled))
			Expect(hook.RunCallCount()).To(Equal(1))
		})
	})

	Context("when the step succeeds", func() {
		BeforeEach(func() {
			step.RunReturns(true, nil)
		})

		It("is successful", func() {
			Expect(stepOk).To(BeTrue())
		})

		It("does not run the abort hook", func() {
			Expect(hook.RunCallCount()).To(Equal(0))
		})
	})

	Context("when the step fails", func() {
		BeforeEach(func() {
			step.RunReturns(false, nil)
		})

		It("is not successful", func() {
			Expect(stepOk).ToNot(BeTrue())
		})

		It("does not run the abort hook", func() {
			Expect(step.RunCallCount()).To(Equal(1))
			Expect(hook.RunCallCount()).To(Equal(0))
		})
	})

	Context("when the step errors", func() {
		disaster := errors.New("disaster")

		BeforeEach(func() {
			step.RunReturns(false, disaster)
		})

		It("returns the error", func() {
			Expect(stepErr).To(Equal(disaster))
		})

		It("does not run the abort hook", func() {
			Expect(step.RunCallCount()).To(Equal(1))
			Expect(hook.RunCallCount()).To(Equal(0))
		})
	})
})
