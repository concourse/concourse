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

var _ = Describe("On Success Step", func() {
	var (
		noError       = BeNil
		errorMatching = MatchError

		stepFactory    *execfakes.FakeStepFactory
		successFactory *execfakes.FakeStepFactory

		step *execfakes.FakeStep
		hook *execfakes.FakeStep

		repo *worker.ArtifactRepository

		onSuccessFactory exec.StepFactory
		onSuccessStep    exec.Step
	)

	BeforeEach(func() {
		stepFactory = &execfakes.FakeStepFactory{}
		successFactory = &execfakes.FakeStepFactory{}

		step = &execfakes.FakeStep{}
		hook = &execfakes.FakeStep{}

		stepFactory.UsingReturns(step)
		successFactory.UsingReturns(hook)

		repo = worker.NewArtifactRepository()

		onSuccessFactory = exec.OnSuccess(stepFactory, successFactory)
		onSuccessStep = onSuccessFactory.Using(repo)
	})

	It("runs the success hook if the step succeeds", func() {
		step.SucceededReturns(true)

		process := ifrit.Background(onSuccessStep)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(hook.RunCallCount).Should(Equal(1))

		Eventually(process.Wait()).Should(Receive(noError()))
	})

	It("provides the step as the previous step to the hook", func() {
		step.SucceededReturns(true)

		process := ifrit.Background(onSuccessStep)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(successFactory.UsingCallCount).Should(Equal(1))

		argsRepo := successFactory.UsingArgsForCall(0)
		Expect(argsRepo).To(Equal(repo))

		Eventually(process.Wait()).Should(Receive(noError()))
	})

	It("does not run the success hook if the step errors", func() {
		step.RunReturns(errors.New("disaster"))

		process := ifrit.Background(onSuccessStep)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(process.Wait()).Should(Receive(errorMatching("disaster")))
		Expect(hook.RunCallCount()).To(Equal(0))
	})

	It("does not run the success hook if the step fails", func() {
		step.SucceededReturns(false)

		process := ifrit.Background(onSuccessStep)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(process.Wait()).Should(Receive(noError()))
		Expect(hook.RunCallCount()).To(Equal(0))
	})

	It("propagates signals to the first step when first step is running", func() {
		step.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)

			<-signals
			return errors.New("interrupted")
		}

		process := ifrit.Background(onSuccessStep)

		process.Signal(os.Kill)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(process.Wait()).Should(Receive(errorMatching("interrupted")))
		Expect(hook.RunCallCount()).To(Equal(0))
	})

	It("propagates signals to the hook when the hook is running", func() {
		step.SucceededReturns(true)

		hook.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)

			<-signals
			return errors.New("interrupted")
		}

		process := ifrit.Background(onSuccessStep)

		process.Signal(os.Kill)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(process.Wait()).Should(Receive(errorMatching("interrupted")))
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

				It("assigns the provided interface to true", func() {
					onSuccessStep.Run(signals, ready)
					Expect(onSuccessStep.Succeeded()).To(BeTrue())
				})
			})

			Context("when step fails", func() {
				BeforeEach(func() {
					step.SucceededReturns(false)
				})

				It("does not run hook and assigns the provided interface to false", func() {
					onSuccessStep.Run(signals, ready)

					Expect(hook.RunCallCount()).To(Equal(0))
					Expect(hook.SucceededCallCount()).To(Equal(0))
					Expect(onSuccessStep.Succeeded()).To(BeFalse())
				})
			})

			Context("when step succeeds and hook fails", func() {
				BeforeEach(func() {
					step.SucceededReturns(true)
					hook.SucceededReturns(false)
				})

				It("assigns the provided interface to false", func() {
					onSuccessStep.Run(signals, ready)
					Expect(onSuccessStep.Succeeded()).To(BeFalse())

					Expect(step.RunCallCount()).To(Equal(1))
					Expect(step.SucceededCallCount()).To(Equal(2))
					Expect(hook.RunCallCount()).To(Equal(1))
					Expect(hook.SucceededCallCount()).To(Equal(1))
				})
			})
		})
	})
})
