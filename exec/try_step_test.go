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
		step = try.Using(nil)
	})

	Describe("Succeeded", func() {
		It("returns true", func() {
			Expect(step.Succeeded()).Should(BeTrue())
		})
	})

	Describe("Run", func() {
		Context("when the inner step is interrupted", func() {
			BeforeEach(func() {
				runStep.RunReturns(ErrInterrupted)
			})

			It("propagates the error", func() {
				err := step.Run(nil, nil)
				Expect(err).To(Equal(ErrInterrupted))
			})
		})

		Context("when the inner step returns any other error", func() {
			BeforeEach(func() {
				runStep.RunReturns(errors.New("some error"))
			})

			It("swallows the error", func() {
				err := step.Run(nil, nil)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
