package exec_test

import (
	"errors"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/fakes"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func successResult(result Success) func(dest interface{}) bool {
	return func(dest interface{}) bool {
		switch x := dest.(type) {
		case *Success:
			*x = result
			return true

		default:
			return false
		}
	}
}

var _ = Describe("Conditional", func() {
	var (
		inStep *fakes.FakeStep
		repo   *SourceRepository

		fakeStepFactory *fakes.FakeStepFactory
		conditional     Conditional

		outStep *fakes.FakeStep

		step    Step
		process ifrit.Process
	)

	BeforeEach(func() {
		inStep = new(fakes.FakeStep)
		repo = NewSourceRepository()

		fakeStepFactory = new(fakes.FakeStepFactory)

		outStep = new(fakes.FakeStep)
		outStep.ResultStub = successResult(true)

		fakeStepFactory.UsingReturns(outStep)

		conditional = Conditional{
			StepFactory: fakeStepFactory,
		}
	})

	JustBeforeEach(func() {
		step = conditional.Using(inStep, repo)
		process = ifrit.Invoke(step)
	})

	itDoesNothing := func() {
		It("succeeds", func() {
			Eventually(process.Wait()).Should(Receive(BeNil()))
		})

		It("does not use the step's artifact source", func() {
			Ω(fakeStepFactory.UsingCallCount()).Should(BeZero())
		})

		Describe("releasing", func() {
			It("does not release the input source", func() {
				Ω(inStep.ReleaseCallCount()).Should(Equal(0))
			})
		})

		Describe("getting the result", func() {
			It("fails", func() {
				var success Success
				Ω(step.Result(&success)).Should(BeFalse())
			})
		})
	}

	itDoesAThing := func() {
		It("succeeds", func() {
			Eventually(process.Wait()).Should(Receive(BeNil()))
		})

		It("uses the step's artifact source", func() {
			Ω(fakeStepFactory.UsingCallCount()).Should(Equal(1))

			step, repo := fakeStepFactory.UsingArgsForCall(0)
			Ω(step).Should(Equal(inStep))
			Ω(repo).Should(Equal(repo))
		})

		Describe("releasing", func() {
			It("releases the output source", func() {
				err := step.Release()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(outStep.ReleaseCallCount()).Should(Equal(1))
			})

			Context("when releasing the output source fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					outStep.ReleaseReturns(disaster)
				})

				It("returns the error", func() {
					Ω(step.Release()).Should(Equal(disaster))
				})
			})
		})
	}

	Context("with no conditions", func() {
		BeforeEach(func() {
			conditional.Conditions = atc.Conditions{}
		})

		Context("when the input source is successful", func() {
			BeforeEach(func() {
				inStep.ResultStub = successResult(true)
			})

			itDoesNothing()
		})

		Context("when the input source failed", func() {
			BeforeEach(func() {
				inStep.ResultStub = successResult(false)
			})

			itDoesNothing()
		})

		Context("when the input source cannot indicate success", func() {
			BeforeEach(func() {
				inStep.ResultReturns(false)
			})

			itDoesNothing()
		})
	})

	Context("with a success condition", func() {
		BeforeEach(func() {
			conditional.Conditions = atc.Conditions{atc.ConditionSuccess}
		})

		Context("when the input source is successful", func() {
			BeforeEach(func() {
				inStep.ResultStub = successResult(true)
			})

			itDoesAThing()
		})

		Context("when the input source failed", func() {
			BeforeEach(func() {
				inStep.ResultStub = successResult(false)
			})

			itDoesNothing()
		})

		Context("when the input source cannot indicate success", func() {
			BeforeEach(func() {
				inStep.ResultReturns(false)
			})

			itDoesNothing()
		})
	})

	Context("with a failure condition", func() {
		BeforeEach(func() {
			conditional.Conditions = atc.Conditions{atc.ConditionFailure}
		})

		Context("when the input source is successful", func() {
			BeforeEach(func() {
				inStep.ResultStub = successResult(true)
			})

			itDoesNothing()
		})

		Context("when the input source failed", func() {
			BeforeEach(func() {
				inStep.ResultStub = successResult(false)
			})

			itDoesAThing()
		})

		Context("when the input source cannot indicate success", func() {
			BeforeEach(func() {
				inStep.ResultReturns(false)
			})

			itDoesNothing()
		})
	})

	Context("with a success or failure condition", func() {
		BeforeEach(func() {
			conditional.Conditions = atc.Conditions{
				atc.ConditionFailure,
				atc.ConditionSuccess,
			}
		})

		Context("when the input source is successful", func() {
			BeforeEach(func() {
				inStep.ResultStub = successResult(true)
			})

			itDoesAThing()
		})

		Context("when the input source failed", func() {
			BeforeEach(func() {
				inStep.ResultStub = successResult(false)
			})

			itDoesAThing()
		})

		Context("when the input source cannot indicate success", func() {
			BeforeEach(func() {
				inStep.ResultReturns(false)
			})

			itDoesNothing()
		})
	})
})
