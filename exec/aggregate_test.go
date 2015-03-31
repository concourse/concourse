package exec_test

import (
	"errors"
	"os"
	"sync"

	. "github.com/concourse/atc/exec"

	"github.com/concourse/atc/exec/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Aggregate", func() {
	var (
		fakeStepA *fakes.FakeStepFactory
		fakeStepB *fakes.FakeStepFactory

		aggregate StepFactory

		inStep *fakes.FakeStep
		repo   *SourceRepository

		outStepA *fakes.FakeStep
		outStepB *fakes.FakeStep

		step    Step
		process ifrit.Process
	)

	BeforeEach(func() {
		fakeStepA = new(fakes.FakeStepFactory)
		fakeStepB = new(fakes.FakeStepFactory)

		aggregate = Aggregate{
			"A": fakeStepA,
			"B": fakeStepB,
		}

		inStep = new(fakes.FakeStep)
		repo = NewSourceRepository()

		outStepA = new(fakes.FakeStep)
		fakeStepA.UsingReturns(outStepA)

		outStepB = new(fakes.FakeStep)
		fakeStepB.UsingReturns(outStepB)
	})

	JustBeforeEach(func() {
		step = aggregate.Using(inStep, repo)
		process = ifrit.Invoke(step)
	})

	It("uses the input source for all steps", func() {
		Ω(fakeStepA.UsingCallCount()).Should(Equal(1))
		step, repo := fakeStepA.UsingArgsForCall(0)
		Ω(step).Should(Equal(inStep))
		Ω(repo).Should(Equal(repo))

		Ω(fakeStepB.UsingCallCount()).Should(Equal(1))
		step, repo = fakeStepB.UsingArgsForCall(0)
		Ω(step).Should(Equal(inStep))
		Ω(repo).Should(Equal(repo))
	})

	It("exits successfully", func() {
		Eventually(process.Wait()).Should(Receive(BeNil()))
	})

	Describe("executing each source", func() {
		BeforeEach(func() {
			wg := new(sync.WaitGroup)
			wg.Add(2)

			outStepA.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				wg.Done()
				wg.Wait()
				close(ready)
				return nil
			}

			outStepB.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				wg.Done()
				wg.Wait()
				close(ready)
				return nil
			}
		})

		It("happens concurrently", func() {
			Ω(outStepA.RunCallCount()).Should(Equal(1))
			Ω(outStepB.RunCallCount()).Should(Equal(1))
		})
	})

	Describe("signalling", func() {
		var receivedSignals chan os.Signal

		BeforeEach(func() {
			receivedSignals = make(chan os.Signal, 2)

			outStepA.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				close(ready)
				receivedSignals <- <-signals
				return nil
			}

			outStepB.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				close(ready)
				receivedSignals <- <-signals
				return nil
			}
		})

		It("propagates to all sources", func() {
			process.Signal(os.Interrupt)

			Eventually(process.Wait()).Should(Receive())

			Ω(receivedSignals).Should(Receive(Equal(os.Interrupt)))
			Ω(receivedSignals).Should(Receive(Equal(os.Interrupt)))
		})
	})

	Context("when sources fail", func() {
		disasterA := errors.New("nope A")
		disasterB := errors.New("nope B")

		BeforeEach(func() {
			outStepA.RunReturns(disasterA)
			outStepB.RunReturns(disasterB)
		})

		It("exits with an error including the original message", func() {
			var err error
			Eventually(process.Wait()).Should(Receive(&err))

			Ω(err.Error()).Should(ContainSubstring("A: nope A"))
			Ω(err.Error()).Should(ContainSubstring("B: nope B"))
		})
	})

	Describe("releasing", func() {
		It("releases all sources", func() {
			err := step.Release()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(outStepA.ReleaseCallCount()).Should(Equal(1))
			Ω(outStepB.ReleaseCallCount()).Should(Equal(1))
		})

		Context("when the sources fail to release", func() {
			disasterA := errors.New("nope A")
			disasterB := errors.New("nope B")

			BeforeEach(func() {
				outStepA.ReleaseReturns(disasterA)
				outStepB.ReleaseReturns(disasterB)
			})

			It("returns an error describing the failures", func() {
				err := step.Release()
				Ω(err).Should(HaveOccurred())

				Ω(err.Error()).Should(ContainSubstring("A: nope A"))
				Ω(err.Error()).Should(ContainSubstring("B: nope B"))
			})
		})
	})

	Describe("getting a result", func() {
		Context("when getting a map of results", func() {
			BeforeEach(func() {
				outStepA.ResultStub = successResult(true)
				outStepB.ResultStub = successResult(false)
			})

			It("collects aggregate results into a map", func() {
				result := map[string]Success{}
				Ω(step.Result(&result)).Should(BeTrue())

				Ω(result["A"]).Should(Equal(Success(true)))
				Ω(result["B"]).Should(Equal(Success(false)))
			})
		})

		Context("when getting a Success result", func() {
			var result Success

			BeforeEach(func() {
				result = false
			})

			Context("and all branches are successful", func() {
				BeforeEach(func() {
					outStepA.ResultStub = successResult(true)
					outStepB.ResultStub = successResult(true)
				})

				It("yields true", func() {
					Ω(step.Result(&result)).Should(BeTrue())
					Ω(result).Should(Equal(Success(true)))
				})
			})

			Context("and some branches are not successful", func() {
				BeforeEach(func() {
					outStepA.ResultStub = successResult(true)
					outStepB.ResultStub = successResult(false)
				})

				It("yields false", func() {
					Ω(step.Result(&result)).Should(BeTrue())
					Ω(result).Should(Equal(Success(false)))
				})
			})

			Context("when some branches do not indicate success", func() {
				BeforeEach(func() {
					outStepA.ResultStub = successResult(true)
					outStepB.ResultReturns(false)
				})

				It("only considers the branches that do", func() {
					Ω(step.Result(&result)).Should(BeTrue())
					Ω(result).Should(Equal(Success(true)))
				})
			})

			Context("when no branches indicate success", func() {
				BeforeEach(func() {
					outStepA.ResultReturns(false)
					outStepB.ResultReturns(false)
				})

				It("returns false", func() {
					Ω(step.Result(&result)).Should(BeFalse())
					Ω(result).Should(Equal(Success(false)))
				})
			})
		})
	})
})
