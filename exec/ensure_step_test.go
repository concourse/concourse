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
		stepFactory *execfakes.FakeStepFactory
		hookFactory *execfakes.FakeStepFactory

		step *execfakes.FakeStep
		hook *execfakes.FakeStep

		previousStep *execfakes.FakeStep

		repo *worker.ArtifactRepository

		ensureFactory exec.StepFactory
		ensureStep    exec.Step
	)

	BeforeEach(func() {
		stepFactory = &execfakes.FakeStepFactory{}
		hookFactory = &execfakes.FakeStepFactory{}

		step = &execfakes.FakeStep{}
		hook = &execfakes.FakeStep{}

		step.RunStub = func(ctx context.Context) error {
			return ctx.Err()
		}

		hook.RunStub = func(ctx context.Context) error {
			return ctx.Err()
		}

		previousStep = &execfakes.FakeStep{}

		stepFactory.UsingReturns(step)
		hookFactory.UsingReturns(hook)

		repo = worker.NewArtifactRepository()

		ensureFactory = exec.Ensure(stepFactory, hookFactory)
		ensureStep = ensureFactory.Using(repo)
	})

	It("runs the ensure hook if the step succeeds", func() {
		step.SucceededReturns(true)

		Expect(ensureStep.Run(context.TODO())).To(Succeed())

		Expect(step.RunCallCount()).To(Equal(1))
		Expect(hook.RunCallCount()).To(Equal(1))
	})

	It("runs the ensure hook if the step fails", func() {
		step.SucceededReturns(false)

		Expect(ensureStep.Run(context.TODO())).To(Succeed())

		Expect(step.RunCallCount()).To(Equal(1))
		Expect(hook.RunCallCount()).To(Equal(1))
	})

	It("provides the step as the previous step to the hook", func() {
		Expect(ensureStep.Run(context.TODO())).To(Succeed())

		Expect(step.RunCallCount()).To(Equal(1))

		Expect(hookFactory.UsingCallCount()).To(Equal(1))

		argsRepo := hookFactory.UsingArgsForCall(0)
		Expect(argsRepo).To(Equal(repo))
	})

	It("runs the ensured hook even if the step errors", func() {
		step.RunReturns(errors.New("disaster"))

		err := ensureStep.Run(context.TODO())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("disaster"))

		Expect(step.RunCallCount()).To(Equal(1))
		Expect(hook.RunCallCount()).To(Equal(1))
	})

	It("allows canceling the first step, and runs the hook", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		Expect(ensureStep.Run(ctx)).To(Equal(context.Canceled))
		Expect(step.RunCallCount()).To(Equal(1))
		Expect(hook.RunCallCount()).To(Equal(1))
		Expect(step.RunArgsForCall(0).Err()).To(Equal(context.Canceled))
		Expect(hook.RunArgsForCall(0).Err()).ToNot(HaveOccurred())
	})

	It("allows canceling the hook if the first step has not been canceled", func() {
		ctx, cancel := context.WithCancel(context.Background())

		Expect(ensureStep.Run(ctx)).To(Succeed())
		Expect(step.RunCallCount()).To(Equal(1))
		Expect(hook.RunCallCount()).To(Equal(1))
		Expect(step.RunArgsForCall(0).Err()).ToNot(HaveOccurred())
		Expect(hook.RunArgsForCall(0).Err()).ToNot(HaveOccurred())
		cancel()
		Expect(step.RunArgsForCall(0).Err()).To(Equal(context.Canceled))
		Expect(hook.RunArgsForCall(0).Err()).To(Equal(context.Canceled))
	})

	Describe("Succeeded", func() {
		Context("when the provided interface is type Success", func() {
			Context("when both step and hook succeed", func() {
				BeforeEach(func() {
					step.SucceededReturns(true)
					hook.SucceededReturns(true)
				})

				It("succeeds", func() {
					ensureStep.Run(context.TODO())
					Expect(ensureStep.Succeeded()).To(BeTrue())
				})
			})

			Context("when step succeeds and hook fails", func() {
				BeforeEach(func() {
					step.SucceededReturns(true)
					hook.SucceededReturns(false)
				})

				It("does not succeed", func() {
					ensureStep.Run(context.TODO())
					Expect(ensureStep.Succeeded()).To(BeFalse())
				})
			})

			Context("when step fails and hook succeeds", func() {
				BeforeEach(func() {
					step.SucceededReturns(false)
					hook.SucceededReturns(true)
				})

				It("does not succeed", func() {
					Expect(ensureStep.Succeeded()).To(BeFalse())
				})
			})

			Context("when step succeeds and hook fails", func() {
				BeforeEach(func() {
					step.SucceededReturns(false)
					hook.SucceededReturns(false)
				})

				It("does not succeed", func() {
					Expect(ensureStep.Succeeded()).To(BeFalse())
				})
			})
		})
	})
})
