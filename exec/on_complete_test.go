package exec_test

import (
	"errors"

	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/fakes"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("OnComplete", func() {
	var (
		wrappedStep *fakes.FakeStep

		inSource  *fakes.FakeArtifactSource
		outSource *fakes.FakeArtifactSource

		step     Step
		callback CompleteCallback

		source ArtifactSource
	)

	BeforeEach(func() {
		wrappedStep = new(fakes.FakeStep)

		inSource = new(fakes.FakeArtifactSource)

		outSource = new(fakes.FakeArtifactSource)
		wrappedStep.UsingReturns(outSource)

		callback = CallbackFunc(func(error, ArtifactSource) {})
	})

	JustBeforeEach(func() {
		step = OnComplete(wrappedStep, callback)
	})

	Describe("Using", func() {
		JustBeforeEach(func() {
			source = step.Using(inSource)
		})

		It("uses the input for the substep", func() {
			Ω(wrappedStep.UsingCallCount()).Should(Equal(1))
			Ω(wrappedStep.UsingArgsForCall(0)).Should(Equal(inSource))
		})

		Describe("Invoking", func() {
			var process ifrit.Process

			JustBeforeEach(func() {
				process = ifrit.Invoke(source)
			})

			Context("when the wrapped step succeeds", func() {
				var calledBack chan struct{}

				BeforeEach(func() {
					outSource.RunReturns(nil)

					calledBack = make(chan struct{})
					callback = CallbackFunc(func(err error, source ArtifactSource) {
						Ω(err).ShouldNot(HaveOccurred())
						Ω(source).Should(Equal(outSource))
						close(calledBack)
					})
				})

				It("succeeds", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))
				})

				It("invokes the callback with no error and the source", func() {
					Eventually(calledBack).Should(BeClosed())
				})
			})

			Context("when the wrapped step fails", func() {
				disaster := errors.New("nope")

				var calledBack chan struct{}

				BeforeEach(func() {
					outSource.RunReturns(disaster)

					calledBack = make(chan struct{})
					callback = CallbackFunc(func(err error, source ArtifactSource) {
						Ω(err).Should(Equal(disaster))
						Ω(source).Should(Equal(outSource))
						close(calledBack)
					})
				})

				It("propagates the failure", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
				})

				It("invokes the callback with the error and the source", func() {
					Eventually(calledBack).Should(BeClosed())
				})
			})
		})
	})

	Context("when", func() {

	})
})
