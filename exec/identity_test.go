package exec_test

import (
	"os"

	. "github.com/concourse/atc/exec"

	"github.com/concourse/atc/exec/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Identity", func() {
	var (
		inStep *fakes.FakeStep
		repo   *SourceRepository

		identity Identity

		step Step
	)

	BeforeEach(func() {
		identity = Identity{}

		inStep = new(fakes.FakeStep)
		repo = NewSourceRepository()
	})

	JustBeforeEach(func() {
		step = identity.Using(inStep, repo)
	})

	Describe("Run", func() {
		It("is a no-op", func() {
			ready := make(chan struct{})
			signals := make(chan os.Signal)

			err := step.Run(signals, ready)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(inStep.RunCallCount()).Should(BeZero())
		})
	})

	Describe("Result", func() {
		It("calls through to the input source", func() {
			var result int
			step.Result(&result)

			Ω(inStep.ResultCallCount()).Should(Equal(1))
			Ω(inStep.ResultArgsForCall(0)).Should(Equal(&result))
		})
	})
})
