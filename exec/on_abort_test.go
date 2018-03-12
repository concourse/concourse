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

var _ = Describe("On Abort Step", func() {
	var (
		ctx    context.Context
		cancel func()

		step *execfakes.FakeStep
		hook *execfakes.FakeStep

		repo  *worker.ArtifactRepository
		state *execfakes.FakeRunState

		onAbortStep exec.Step

		stepErr error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		step = &execfakes.FakeStep{}
		hook = &execfakes.FakeStep{}

		repo = worker.NewArtifactRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactsReturns(repo)

		onAbortStep = exec.OnAbort(step, hook)

		stepErr = nil
	})

	JustBeforeEach(func() {
		stepErr = onAbortStep.Run(ctx, state)
	})

	Context("when the step is aborted", func() {
		BeforeEach(func() {
			step.RunReturns(context.Canceled)
		})

		It("runs the abort hook", func() {
			Expect(stepErr).To(Equal(context.Canceled))
			Expect(hook.RunCallCount()).To(Equal(1))
		})
	})

	Context("when the step succeeds", func() {
		BeforeEach(func() {
			step.SucceededReturns(true)
		})

		It("is successful", func() {
			Expect(onAbortStep.Succeeded()).To(BeTrue())
		})

		It("does not run the abort hook", func() {
			Expect(hook.RunCallCount()).To(Equal(0))
		})
	})

	Context("when the step fails", func() {
		BeforeEach(func() {
			step.SucceededReturns(false)
		})

		It("is not successful", func() {
			Expect(onAbortStep.Succeeded()).ToNot(BeTrue())
		})

		It("does not run the abort hook", func() {
			Expect(step.RunCallCount()).To(Equal(1))
			Expect(hook.RunCallCount()).To(Equal(0))
		})
	})

	Context("when the step errors", func() {
		disaster := errors.New("disaster")

		BeforeEach(func() {
			step.RunReturns(disaster)
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
