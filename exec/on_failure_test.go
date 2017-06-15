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

var _ = Describe("On Failure Step", func() {
	var (
		stepFactory    *execfakes.FakeStepFactory
		failureFactory *execfakes.FakeStepFactory

		step *execfakes.FakeStep
		hook *execfakes.FakeStep

		previousStep *execfakes.FakeStep

		repo *worker.ArtifactRepository

		onFailureFactory exec.StepFactory
		onFailureStep    exec.Step
	)

	BeforeEach(func() {
		stepFactory = &execfakes.FakeStepFactory{}
		failureFactory = &execfakes.FakeStepFactory{}

		step = &execfakes.FakeStep{}
		hook = &execfakes.FakeStep{}

		previousStep = &execfakes.FakeStep{}

		stepFactory.UsingReturns(step)
		failureFactory.UsingReturns(hook)

		repo = worker.NewArtifactRepository()

		onFailureFactory = exec.OnFailure(stepFactory, failureFactory)
		onFailureStep = onFailureFactory.Using(previousStep, repo)
	})

	It("runs the failure hook if step fails", func() {
		step.ResultStub = successResult(false)

		process := ifrit.Background(onFailureStep)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(hook.RunCallCount).Should(Equal(1))

		Eventually(process.Wait()).Should(Receive(noError()))
	})

	It("provides the step as the previous step to the hook", func() {
		step.ResultStub = successResult(false)

		process := ifrit.Background(onFailureStep)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(failureFactory.UsingCallCount).Should(Equal(1))

		argsPrev, argsRepo := failureFactory.UsingArgsForCall(0)
		Expect(argsPrev).To(Equal(step))
		Expect(argsRepo).To(Equal(repo))

		Eventually(process.Wait()).Should(Receive(noError()))
	})

	It("does not run the failure hook if the step errors", func() {
		step.RunReturns(errors.New("disaster"))

		process := ifrit.Background(onFailureStep)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(process.Wait()).Should(Receive(errorMatching("disaster")))
		Expect(hook.RunCallCount()).To(Equal(0))
	})

	It("does not run the failure hook if the step succeeds", func() {
		step.ResultStub = successResult(true)

		process := ifrit.Background(onFailureStep)

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

		process := ifrit.Background(onFailureStep)

		process.Signal(os.Kill)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(process.Wait()).Should(Receive(errorMatching("interrupted")))
		Expect(hook.RunCallCount()).To(Equal(0))
	})

	It("propagates signals to the hook when the hook is running", func() {
		step.ResultStub = successResult(false)

		hook.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)

			<-signals
			return errors.New("interrupted")
		}

		process := ifrit.Background(onFailureStep)

		process.Signal(os.Kill)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(process.Wait()).Should(Receive(errorMatching("interrupted")))
		Expect(hook.RunCallCount()).To(Equal(1))
	})

	Describe("Result", func() {
		Context("when the provided interface is type Success", func() {
			var signals chan os.Signal
			var ready chan struct{}

			BeforeEach(func() {
				signals = make(chan os.Signal, 1)
				ready = make(chan struct{}, 1)
			})

			Context("when step fails and hook fails", func() {
				BeforeEach(func() {
					step.ResultStub = successResult(false)
					hook.ResultStub = successResult(false)
				})

				It("assigns the provided interface to false", func() {
					var succeeded exec.Success
					onFailureStep.Run(signals, ready)
					onFailureStep.Result(&succeeded)

					Expect(bool(succeeded)).To(BeFalse())
				})
			})

			Context("when step fails and hook succeeds", func() {
				BeforeEach(func() {
					step.ResultStub = successResult(false)
					hook.ResultStub = successResult(true)
				})

				It("assigns the provided interface to false", func() {
					var succeeded exec.Success
					onFailureStep.Run(signals, ready)
					onFailureStep.Result(&succeeded)

					Expect(bool(succeeded)).To(BeFalse())
				})
			})

			Context("when step succeeds", func() {
				BeforeEach(func() {
					step.ResultStub = successResult(true)
				})

				It("never runs hook", func() {
					var succeeded exec.Success
					onFailureStep.Run(signals, ready)
					onFailureStep.Result(&succeeded)
					Expect(hook.RunCallCount()).To(Equal(0))
					Expect(hook.ResultCallCount()).To(Equal(0))
					Expect(bool(succeeded)).To(BeTrue())
				})
			})

			Context("when step fails and hook fails", func() {
				BeforeEach(func() {
					step.ResultStub = successResult(false)
					hook.ResultStub = successResult(false)
				})

				It("doesn't indicate success", func() {
					var succeeded exec.Success
					onFailureStep.Run(signals, ready)
					onFailureStep.Result(&succeeded)
					Expect(step.RunCallCount()).To(Equal(1))
					Expect(step.ResultCallCount()).To(Equal(1))
					Expect(hook.RunCallCount()).To(Equal(1))
					Expect(hook.ResultCallCount()).To(Equal(0))
					Expect(bool(succeeded)).To(BeFalse())
				})
			})

			Context("when step fails and hook succeeds", func() {
				BeforeEach(func() {
					step.ResultStub = successResult(false)
					hook.ResultStub = successResult(true)
				})

				It("doesn't indicate success", func() {
					var succeeded exec.Success
					onFailureStep.Run(signals, ready)
					onFailureStep.Result(&succeeded)
					Expect(step.RunCallCount()).To(Equal(1))
					Expect(step.ResultCallCount()).To(Equal(1))
					Expect(hook.RunCallCount()).To(Equal(1))
					Expect(hook.ResultCallCount()).To(Equal(0))
					Expect(bool(succeeded)).To(BeFalse())
				})
			})
		})
	})
})
