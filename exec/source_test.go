package exec_test

import (
	"errors"
	"os"

	. "github.com/concourse/atc/exec"

	"github.com/concourse/atc/exec/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Source", func() {
	var (
		name            SourceName
		fakeStepFactory *fakes.FakeStepFactory

		inStep *fakes.FakeStep
		repo   *SourceRepository

		source Source

		fakeStep *fakes.FakeStep

		step Step
	)

	BeforeEach(func() {
		name = "some-name"

		fakeStepFactory = new(fakes.FakeStepFactory)

		fakeStep = new(fakes.FakeStep)
		fakeStepFactory.UsingReturns(fakeStep)

		source = Source{
			Name:        name,
			StepFactory: fakeStepFactory,
		}

		inStep = new(fakes.FakeStep)
		repo = NewSourceRepository()
	})

	JustBeforeEach(func() {
		step = source.Using(inStep, repo)
	})

	Describe("Run", func() {
		var runErr error

		JustBeforeEach(func() {
			ready := make(chan struct{})
			signals := make(chan os.Signal)

			runErr = step.Run(signals, ready)
		})

		Context("when the sub-step fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeStep.RunReturns(disaster)
			})

			It("returns the error", func() {
				Ω(runErr).Should(Equal(disaster))
			})

			It("does not register it as a source", func() {
				_, found := repo.SourceFor("some-name")
				Ω(found).Should(BeFalse())
			})
		})

		Context("when the sub-step succeeds", func() {
			BeforeEach(func() {
				fakeStep.RunReturns(nil)
			})

			It("exits successfully", func() {
				Ω(runErr).Should(BeNil())
			})

			It("registers the step as a source", func() {
				registeredSource, found := repo.SourceFor("some-name")
				Ω(found).Should(BeTrue())

				Ω(registeredSource).Should(Equal(fakeStep))
			})
		})
	})
})
