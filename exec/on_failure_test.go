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

var _ = Describe("On Failure Step", func() {
	var (
		ctx    context.Context
		cancel func()

		stepFactory    *execfakes.FakeStepFactory
		failureFactory *execfakes.FakeStepFactory

		step *execfakes.FakeStep
		hook *execfakes.FakeStep

		repo *worker.ArtifactRepository

		onFailureFactory exec.StepFactory
		onFailureStep    exec.Step

		stepErr error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		stepFactory = &execfakes.FakeStepFactory{}
		failureFactory = &execfakes.FakeStepFactory{}

		step = &execfakes.FakeStep{}
		hook = &execfakes.FakeStep{}

		stepFactory.UsingReturns(step)
		failureFactory.UsingReturns(hook)

		repo = worker.NewArtifactRepository()

		onFailureFactory = exec.OnFailure(stepFactory, failureFactory)
		onFailureStep = onFailureFactory.Using(repo)
	})

	JustBeforeEach(func() {
		stepErr = onFailureStep.Run(ctx)
	})

	Context("when the step fails", func() {
		BeforeEach(func() {
			step.SucceededReturns(false)
		})

		It("runs the failure hook", func() {
			Expect(step.RunCallCount()).To(Equal(1))
			Expect(hook.RunCallCount()).To(Equal(1))
		})

		It("constructs the hook with the artifact repo", func() {
			Expect(failureFactory.UsingCallCount()).To(Equal(1))

			argsRepo := failureFactory.UsingArgsForCall(0)
			Expect(argsRepo).To(Equal(repo))
		})

		It("propagates the context to the hook", func() {
			Expect(hook.RunArgsForCall(0)).To(Equal(ctx))
		})

		It("succeeds", func() {
			Expect(stepErr).ToNot(HaveOccurred())
		})
	})

	Context("when the step errors", func() {
		disaster := errors.New("disaster")

		BeforeEach(func() {
			step.RunReturns(disaster)
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
			step.SucceededReturns(true)
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
		Expect(step.RunArgsForCall(0)).To(Equal(ctx))
	})

	Describe("Succeeded", func() {
		Context("when step fails and hook fails", func() {
			BeforeEach(func() {
				step.SucceededReturns(false)
				hook.SucceededReturns(false)
			})

			It("returns false", func() {
				Expect(onFailureStep.Succeeded()).To(BeFalse())
			})
		})

		Context("when step fails and hook succeeds", func() {
			BeforeEach(func() {
				step.SucceededReturns(false)
				hook.SucceededReturns(true)
			})

			It("returns false", func() {
				Expect(onFailureStep.Succeeded()).To(BeFalse())
			})
		})

		Context("when step succeeds", func() {
			BeforeEach(func() {
				step.SucceededReturns(true)
			})

			It("returns true", func() {
				Expect(onFailureStep.Succeeded()).To(BeTrue())
			})
		})
	})
})
