package exec_test

import (
	"context"
	"errors"
	"sync"

	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/worker"

	"github.com/concourse/atc/exec/execfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Aggregate", func() {
	var (
		ctx    context.Context
		cancel func()

		fakeStepA *execfakes.FakeStep
		fakeStepB *execfakes.FakeStep

		inStep *execfakes.FakeStep
		repo   *worker.ArtifactRepository
		state  *execfakes.FakeRunState

		step    Step
		stepErr error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		fakeStepA = new(execfakes.FakeStep)
		fakeStepB = new(execfakes.FakeStep)

		step = AggregateStep{
			fakeStepA,
			fakeStepB,
		}

		inStep = new(execfakes.FakeStep)
		repo = worker.NewArtifactRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactsReturns(repo)
	})

	JustBeforeEach(func() {
		stepErr = step.Run(ctx, state)
	})

	It("succeeds", func() {
		Expect(stepErr).ToNot(HaveOccurred())
	})

	It("passes the artifact repo to all steps", func() {
		Expect(fakeStepA.RunCallCount()).To(Equal(1))
		_, repo := fakeStepA.RunArgsForCall(0)
		Expect(repo).To(Equal(repo))

		Expect(fakeStepB.RunCallCount()).To(Equal(1))
		_, repo = fakeStepB.RunArgsForCall(0)
		Expect(repo).To(Equal(repo))
	})

	Describe("executing each source", func() {
		BeforeEach(func() {
			wg := new(sync.WaitGroup)
			wg.Add(2)

			fakeStepA.RunStub = func(context.Context, RunState) error {
				wg.Done()
				wg.Wait()
				return nil
			}

			fakeStepB.RunStub = func(context.Context, RunState) error {
				wg.Done()
				wg.Wait()
				return nil
			}
		})

		It("happens concurrently", func() {
			Expect(fakeStepA.RunCallCount()).To(Equal(1))
			Expect(fakeStepB.RunCallCount()).To(Equal(1))
		})
	})

	Describe("canceling", func() {
		BeforeEach(func() {
			cancel()
		})

		It("cancels each substep", func() {
			ctx, _ := fakeStepA.RunArgsForCall(0)
			Expect(ctx.Err()).To(Equal(context.Canceled))
			ctx, _ = fakeStepB.RunArgsForCall(0)
			Expect(ctx.Err()).To(Equal(context.Canceled))
		})

		It("returns ctx.Err()", func() {
			Expect(stepErr).To(Equal(context.Canceled))
		})
	})

	Context("when sources fail", func() {
		disasterA := errors.New("nope A")
		disasterB := errors.New("nope B")

		BeforeEach(func() {
			fakeStepA.RunReturns(disasterA)
			fakeStepB.RunReturns(disasterB)
		})

		It("exits with an error including the original message", func() {
			Expect(stepErr.Error()).To(ContainSubstring("nope A"))
			Expect(stepErr.Error()).To(ContainSubstring("nope B"))
		})
	})

	Describe("Succeeded", func() {
		Context("when all sources are successful", func() {
			BeforeEach(func() {
				fakeStepA.SucceededReturns(true)
				fakeStepB.SucceededReturns(true)
			})

			It("yields true", func() {
				Expect(step.Succeeded()).To(BeTrue())
			})
		})

		Context("and some branches are not successful", func() {
			BeforeEach(func() {
				fakeStepA.SucceededReturns(true)
				fakeStepB.SucceededReturns(false)
			})

			It("yields false", func() {
				Expect(step.Succeeded()).To(BeFalse())
			})
		})

		Context("when no branches indicate success", func() {
			BeforeEach(func() {
				fakeStepA.SucceededReturns(false)
				fakeStepB.SucceededReturns(false)
			})

			It("returns false", func() {
				Expect(step.Succeeded()).To(BeFalse())
			})
		})

		Context("when there are no branches", func() {
			BeforeEach(func() {
				step = AggregateStep{}
			})

			It("returns true", func() {
				Expect(step.Succeeded()).To(BeTrue())
			})
		})
	})
})
