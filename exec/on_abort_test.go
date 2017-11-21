package exec_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/tedsuo/ifrit"

	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/execfakes"
	"github.com/concourse/atc/worker"
)

var _ = Describe("On Abort Step", func() {
	var (
		noError       = BeNil
		errorMatching = MatchError

		stepFactory  *execfakes.FakeStepFactory
		abortFactory *execfakes.FakeStepFactory

		step *execfakes.FakeStep
		hook *execfakes.FakeStep

		repo *worker.ArtifactRepository

		onAbortFactory exec.StepFactory
		onAbortStep    exec.Step
	)

	BeforeEach(func() {
		stepFactory = &execfakes.FakeStepFactory{}
		abortFactory = &execfakes.FakeStepFactory{}

		step = &execfakes.FakeStep{}
		hook = &execfakes.FakeStep{}

		stepFactory.UsingReturns(step)
		abortFactory.UsingReturns(hook)

		repo = worker.NewArtifactRepository()

		onAbortFactory = exec.OnAbort(stepFactory, abortFactory)
		onAbortStep = onAbortFactory.Using(repo)
	})

	Context("When running a build step that has an abort hook", func() {
		Context("When abort is triggered", func() {
			It("runs the abort hook if step aborted", func() {
				step.RunReturns(exec.ErrInterrupted)

				process := ifrit.Background(onAbortStep)
				Eventually(step.RunCallCount).Should(Equal(1))
				Eventually(process.Wait()).Should(Receive(errorMatching(ContainSubstring("interrupted"))))
				Expect(hook.RunCallCount()).To(Equal(1))
			})
		})

		Context("When abort is not triggered", func() {
			BeforeEach(func() {
				stepFactory := &execfakes.FakeStepFactory{}
				abortFactory := &execfakes.FakeStepFactory{}

				step = &execfakes.FakeStep{}
				hook = &execfakes.FakeStep{}

				stepFactory.UsingReturns(step)
				abortFactory.UsingReturns(hook)

				repo = worker.NewArtifactRepository()

				onAbortFactory := exec.OnAbort(stepFactory, abortFactory)
				onAbortStep = onAbortFactory.Using(repo)
			})

			It("should not run abort hook on step success", func() {
				step.SucceededReturns(true)

				process := ifrit.Background(onAbortStep)

				Eventually(step.RunCallCount).Should(Equal(1))
				Eventually(process.Wait()).Should(Receive(noError()))
				Expect(hook.RunCallCount()).To(Equal(0))
			})

			It("should not run abort hook on step failure", func() {
				step.SucceededReturns(false)

				process := ifrit.Background(onAbortStep)

				Eventually(step.RunCallCount).Should(Equal(1))
				Eventually(process.Wait()).Should(Receive(noError()))
				Expect(hook.RunCallCount()).To(Equal(0))
			})

			It("does not run the abort hook if the step errors", func() {
				step.RunReturns(errors.New("disaster"))

				process := ifrit.Background(onAbortStep)

				Eventually(step.RunCallCount).Should(Equal(1))
				Eventually(process.Wait()).Should(Receive(errorMatching(ContainSubstring("disaster"))))
				Expect(hook.RunCallCount()).To(Equal(0))
			})
		})
	})
})
