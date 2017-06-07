package exec_test

import (
	"errors"

	. "github.com/concourse/atc/exec"

	"github.com/concourse/atc/exec/execfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Try Step", func() {
	var (
		fakeStepFactoryStep *execfakes.FakeStepFactory

		runStep *execfakes.FakeStep

		try  StepFactory
		step Step
	)

	BeforeEach(func() {
		fakeStepFactoryStep = new(execfakes.FakeStepFactory)
		runStep = new(execfakes.FakeStep)
		fakeStepFactoryStep.UsingReturns(runStep)

		try = Try(fakeStepFactoryStep)
		step = try.Using(nil, nil)
	})

	Describe("Succeeded", func() {
		Context("when compared against Success", func() {
			var x *Success

			BeforeEach(func() {
				x = new(Success)
			})

			It("returns true", func() {
				result := step.Succeeded(x)
				Expect(result).Should(Equal(true))
			})

			It("assigns the provided interface to Success(true)", func() {
				step.Succeeded(x)
				Expect(*x).Should(Equal(Success(true)))
			})
		})

		Context("when compared against something other than Success", func() {
			const exitCode = 1234

			BeforeEach(func() {
				runStep.ResultStub = func(x interface{}) bool {
					switch v := x.(type) {
					case *ExitStatus:
						*v = ExitStatus(exitCode)
						return true
					default:
						panic("unexpected Succeeded comparison")
					}
				}
			})

			It("deletegates to the inner step", func() {
				x := new(ExitStatus)
				result := step.Succeeded(x)
				Expect(result).Should(Equal(true))
				Expect(*x).Should(Equal(ExitStatus(exitCode)))
			})
		})
	})

	Describe("Run", func() {
		Context("when the inner step is interrupted", func() {
			BeforeEach(func() {
				runStep.ResultStub = successResult(false)
				runStep.RunReturns(ErrInterrupted)
			})

			It("propagates the error", func() {
				err := step.Run(nil, nil)
				Expect(err).To(Equal(ErrInterrupted))
			})
		})

		Context("when the inner step returns any other error", func() {
			BeforeEach(func() {
				runStep.ResultStub = successResult(false)
				runStep.RunReturns(errors.New("some error"))
			})

			It("swallows the error", func() {
				err := step.Run(nil, nil)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
