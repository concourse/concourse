package exec_test

import (
	"errors"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/tedsuo/ifrit"

	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/fakes"
)

var _ = Describe("Ensure Step", func() {
	var (
		stepFactory *fakes.FakeStepFactory
		hookFactory *fakes.FakeStepFactory

		step *fakes.FakeStep
		hook *fakes.FakeStep

		previousStep *fakes.FakeStep

		repo *exec.SourceRepository

		ensureFactory exec.StepFactory
		ensureStep    exec.Step
	)

	BeforeEach(func() {
		stepFactory = &fakes.FakeStepFactory{}
		hookFactory = &fakes.FakeStepFactory{}

		step = &fakes.FakeStep{}
		hook = &fakes.FakeStep{}

		previousStep = &fakes.FakeStep{}

		stepFactory.UsingReturns(step)
		hookFactory.UsingReturns(hook)

		repo = exec.NewSourceRepository()

		ensureFactory = exec.Ensure(stepFactory, hookFactory)
		ensureStep = ensureFactory.Using(previousStep, repo)
	})

	It("runs the ensure hook if the step succeeds", func() {
		step.ResultStub = successResult(true)

		process := ifrit.Background(ensureStep)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(hook.RunCallCount).Should(Equal(1))

		Eventually(process.Wait()).Should(Receive(noError()))
	})

	It("runs the ensure hook if the step fails", func() {
		step.ResultStub = successResult(false)

		process := ifrit.Background(ensureStep)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(hook.RunCallCount).Should(Equal(1))

		Eventually(process.Wait()).Should(Receive(noError()))
	})

	It("provides the step as the previous step to the hook", func() {
		process := ifrit.Background(ensureStep)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(hookFactory.UsingCallCount).Should(Equal(1))

		argsPrev, argsRepo := hookFactory.UsingArgsForCall(0)
		Ω(argsPrev).Should(Equal(step))
		Ω(argsRepo).Should(Equal(repo))

		Eventually(process.Wait()).Should(Receive(noError()))
	})

	It("does not run the ensure hook if the step errors", func() {
		step.RunReturns(errors.New("disaster"))

		process := ifrit.Background(ensureStep)

		Eventually(step.RunCallCount).Should(Equal(1))
		Eventually(process.Wait()).Should(Receive(errorMatching("disaster")))
		Ω(hook.RunCallCount()).Should(Equal(0))
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
		Eventually(process.Wait()).Should(Receive(errorMatching("interrupted")))
		Ω(hook.RunCallCount()).Should(Equal(0))
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
		Eventually(process.Wait()).Should(Receive(errorMatching("interrupted")))
		Ω(hook.RunCallCount()).Should(Equal(1))
	})

	Describe("Result", func() {
		Context("when the provided interface is type Success", func() {
			var signals chan os.Signal
			var ready chan struct{}
			BeforeEach(func() {
				signals = make(chan os.Signal, 1)
				ready = make(chan struct{}, 1)
			})

			Context("when both step and hook succeed", func() {
				BeforeEach(func() {
					step.ResultStub = successResult(true)
					hook.ResultStub = successResult(true)
				})

				It("assigns the provided interface to true", func() {
					var succeeded exec.Success
					ensureStep.Run(signals, ready)
					ensureStep.Result(&succeeded)

					Ω(bool(succeeded)).To(BeTrue())
				})
			})

			Context("when step succeeds and hook fails", func() {
				BeforeEach(func() {
					step.ResultStub = successResult(true)
					hook.ResultStub = successResult(false)
				})

				It("assigns the provided interface to false", func() {
					var succeeded exec.Success
					ensureStep.Run(signals, ready)
					ensureStep.Result(&succeeded)
					Ω(bool(succeeded)).To(BeFalse())
				})
			})

			Context("when step fails and hook succeeds", func() {
				BeforeEach(func() {
					step.ResultStub = successResult(false)
					hook.ResultStub = successResult(true)
				})

				It("assigns the provided interface to false", func() {
					var succeeded exec.Success
					ensureStep.Run(signals, ready)
					ensureStep.Result(&succeeded)
					Ω(bool(succeeded)).To(BeFalse())
				})
			})

			Context("when step succeeds and hook fails", func() {
				BeforeEach(func() {

					step.ResultStub = successResult(false)
					hook.ResultStub = successResult(false)
				})

				It("assigns the provided interface to false", func() {
					var succeeded exec.Success
					ensureStep.Run(signals, ready)
					ensureStep.Result(&succeeded)
					Ω(bool(succeeded)).To(BeFalse())
				})
			})
		})

		Describe("Release", func() {
			var (
				signals chan os.Signal
				ready   chan struct{}
			)

			Context("when both step and hook are run", func() {
				BeforeEach(func() {
					signals = make(chan os.Signal, 1)
					ready = make(chan struct{}, 1)

					step.ResultStub = successResult(true)
				})
				It("calls release on both step and hook", func() {
					ensureStep.Run(signals, ready)
					ensureStep.Release()
					Ω(step.ReleaseCallCount()).Should(Equal(1))
					Ω(hook.ReleaseCallCount()).Should(Equal(1))
				})
			})
		})
	})
})
