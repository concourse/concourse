package exec_test

import (
	"errors"
	"os"
	"sync"

	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/worker"

	"github.com/concourse/atc/exec/execfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Aggregate", func() {
	var (
		fakeStepA *execfakes.FakeStepFactory
		fakeStepB *execfakes.FakeStepFactory

		aggregate StepFactory

		inStep *execfakes.FakeStep
		repo   *worker.ArtifactRepository

		outStepA *execfakes.FakeStep
		outStepB *execfakes.FakeStep

		step    Step
		process ifrit.Process
	)

	BeforeEach(func() {
		fakeStepA = new(execfakes.FakeStepFactory)
		fakeStepB = new(execfakes.FakeStepFactory)

		aggregate = Aggregate{
			fakeStepA,
			fakeStepB,
		}

		inStep = new(execfakes.FakeStep)
		repo = worker.NewArtifactRepository()

		outStepA = new(execfakes.FakeStep)
		fakeStepA.UsingReturns(outStepA)

		outStepB = new(execfakes.FakeStep)
		fakeStepB.UsingReturns(outStepB)
	})

	JustBeforeEach(func() {
		step = aggregate.Using(repo)
		process = ifrit.Invoke(step)
	})

	It("uses the input source for all steps", func() {
		Expect(fakeStepA.UsingCallCount()).To(Equal(1))
		repo := fakeStepA.UsingArgsForCall(0)
		Expect(repo).To(Equal(repo))

		Expect(fakeStepB.UsingCallCount()).To(Equal(1))
		repo = fakeStepB.UsingArgsForCall(0)
		Expect(repo).To(Equal(repo))
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
			Expect(outStepA.RunCallCount()).To(Equal(1))
			Expect(outStepB.RunCallCount()).To(Equal(1))
		})
	})

	Describe("signalling", func() {
		var receivedSignals chan os.Signal
		var actuallyExit chan struct{}

		BeforeEach(func() {
			receivedSignals = make(chan os.Signal, 2)
			actuallyExit = make(chan struct{}, 1)

			outStepA.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				close(ready)
				receivedSignals <- <-signals
				<-actuallyExit
				return ErrInterrupted
			}

			outStepB.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				close(ready)
				receivedSignals <- <-signals
				<-actuallyExit
				return ErrInterrupted
			}
		})

		It("returns ErrInterrupted", func() {
			process.Signal(os.Interrupt)

			Eventually(receivedSignals).Should(Receive(Equal(os.Interrupt)))
			Eventually(receivedSignals).Should(Receive(Equal(os.Interrupt)))
			Consistently(process.Wait()).ShouldNot(Receive())
			close(actuallyExit)
			Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))
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

			Expect(err.Error()).To(ContainSubstring("nope A"))
			Expect(err.Error()).To(ContainSubstring("nope B"))
		})
	})

	Describe("Succeeded", func() {
		Context("when all sources are successful", func() {
			BeforeEach(func() {
				outStepA.SucceededReturns(true)
				outStepB.SucceededReturns(true)
			})

			It("yields true", func() {
				Expect(step.Succeeded()).To(BeTrue())
			})
		})

		Context("and some branches are not successful", func() {
			BeforeEach(func() {
				outStepA.SucceededReturns(true)
				outStepB.SucceededReturns(false)
			})

			It("yields false", func() {
				Expect(step.Succeeded()).To(BeFalse())
			})
		})

		//		Context("when some branches do not indicate success", func() {
		//			BeforeEach(func() {
		//				outStepA.ResultStub = successResult(true)
		//				outStepB.ResultReturns(false)
		//			})
		//
		//			It("only considers the branches that do", func() {
		//				Expect(step.Succeeded(&result)).To(BeTrue())
		//				Expect(result).To(Equal(Success(true)))
		//			})
		//		})

		Context("when no branches indicate success", func() {
			BeforeEach(func() {
				outStepA.SucceededReturns(false)
				outStepB.SucceededReturns(false)
			})

			It("returns false", func() {
				Expect(step.Succeeded()).To(BeFalse())
			})
		})

		Context("when there are no branches", func() {
			BeforeEach(func() {
				aggregate = Aggregate{}
			})

			It("returns true", func() {
				Expect(step.Succeeded()).To(BeTrue())
			})
		})
	})
})
