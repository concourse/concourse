package exec_test

import (
	"errors"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/tedsuo/ifrit"

	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/execfakes"
	"github.com/concourse/atc/worker"
)

var _ = Describe("Ensure Step", func() {
	var (
		noError       = BeNil
		errorMatching = MatchError

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

		previousStep = &execfakes.FakeStep{}

		stepFactory.UsingReturns(step)
		hookFactory.UsingReturns(hook)

		repo = worker.NewArtifactRepository()

		ensureFactory = exec.Ensure(stepFactory, hookFactory)
		ensureStep = ensureFactory.Using(repo)
	})

	It("runs the ensure hook if the step succeeds", func() {
		step.SucceededReturns(true)

		process := ifrit.Background(ensureStep)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(hook.RunCallCount).Should(Equal(1))

		Eventually(process.Wait()).Should(Receive(noError()))
	})

	It("runs the ensure hook if the step fails", func() {
		step.SucceededReturns(false)

		process := ifrit.Background(ensureStep)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(hook.RunCallCount).Should(Equal(1))

		Eventually(process.Wait()).Should(Receive(noError()))
	})

	It("provides the step as the previous step to the hook", func() {
		process := ifrit.Background(ensureStep)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(hookFactory.UsingCallCount).Should(Equal(1))

		argsRepo := hookFactory.UsingArgsForCall(0)
		Expect(argsRepo).To(Equal(repo))

		Eventually(process.Wait()).Should(Receive(noError()))
	})

	It("runs the ensured hook even if the step errors", func() {
		step.RunReturns(errors.New("disaster"))

		process := ifrit.Background(ensureStep)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(process.Wait()).Should(Receive(errorMatching(ContainSubstring("disaster"))))

		Expect(hook.RunCallCount()).To(Equal(1))
	})

	It("propagates signals to the first step when first step is running", func() {
		step.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)

			<-signals
			return errors.New("interrupted")
		}

		process := ifrit.Background(ensureStep)

		process.Signal(os.Kill)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(process.Wait()).Should(Receive(errorMatching(ContainSubstring("interrupted"))))

		Expect(hook.RunCallCount()).To(Equal(1))
	})

	It("propagates signals to the hook when the hook is running", func() {
		hook.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)

			<-signals
			return errors.New("interrupted")
		}

		process := ifrit.Background(ensureStep)

		process.Signal(os.Kill)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(process.Wait()).Should(Receive(errorMatching(ContainSubstring("interrupted"))))

		Expect(hook.RunCallCount()).To(Equal(1))
	})

	Describe("Succeeded", func() {
		Context("when the provided interface is type Success", func() {
			var signals chan os.Signal
			var ready chan struct{}
			BeforeEach(func() {
				signals = make(chan os.Signal, 1)
				ready = make(chan struct{}, 1)
			})

			Context("when both step and hook succeed", func() {
				BeforeEach(func() {
					step.SucceededReturns(true)
					hook.SucceededReturns(true)
				})

				It("succeeds", func() {
					ensureStep.Run(signals, ready)
					Expect(ensureStep.Succeeded()).To(BeTrue())
				})
			})

			Context("when step succeeds and hook fails", func() {
				BeforeEach(func() {
					step.SucceededReturns(true)
					hook.SucceededReturns(false)
				})

				It("does not succeed", func() {
					ensureStep.Run(signals, ready)
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
