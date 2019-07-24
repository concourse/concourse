package exec_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	. "github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Parallel", func() {
	var (
		ctx    context.Context
		cancel func()

		fakeStepA *execfakes.FakeStep
		fakeStepB *execfakes.FakeStep
		fakeSteps []Step

		repo  *build.Repository
		state *execfakes.FakeRunState

		step    Step
		stepErr error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		fakeStepA = new(execfakes.FakeStep)
		fakeStepB = new(execfakes.FakeStep)
		fakeSteps = []Step{fakeStepA, fakeStepB}

		step = InParallel(fakeSteps, len(fakeSteps), false)

		repo = build.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactsReturns(repo)
	})

	AfterEach(func() {
		cancel()
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

	Describe("executing each step", func() {
		Context("when not constrained by parallel limit", func() {
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

		Context("when parallel limit is 1", func() {
			BeforeEach(func() {
				step = InParallel(fakeSteps, 1, false)
				ch := make(chan struct{}, 1)

				fakeStepA.RunStub = func(context.Context, RunState) error {
					time.Sleep(10 * time.Millisecond)
					ch <- struct{}{}
					return nil
				}

				fakeStepB.RunStub = func(context.Context, RunState) error {
					defer GinkgoRecover()

					select {
					case <-ch:
					default:
						Fail("step B started before step A could complete")
					}
					return nil
				}
			})

			It("happens sequentially", func() {
				Expect(fakeStepA.RunCallCount()).To(Equal(1))
				Expect(fakeStepB.RunCallCount()).To(Equal(1))
			})
		})
	})

	Describe("canceling", func() {
		BeforeEach(func() {
			wg := new(sync.WaitGroup)
			wg.Add(2)

			fakeStepA.RunStub = func(context.Context, RunState) error {
				wg.Done()
				return nil
			}

			fakeStepB.RunStub = func(context.Context, RunState) error {
				wg.Done()
				wg.Wait()
				cancel()
				return nil
			}
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

		Context("when there are steps pending execution", func() {
			BeforeEach(func() {
				step = InParallel(fakeSteps, 1, false)

				fakeStepA.RunStub = func(context.Context, RunState) error {
					cancel()
					return nil
				}

				fakeStepB.RunStub = func(context.Context, RunState) error {
					return nil
				}
			})

			It("returns ctx.Err()", func() {
				Expect(stepErr).To(Equal(context.Canceled))
			})

			It("does not execute the remaining steps", func() {
				ctx, _ := fakeStepA.RunArgsForCall(0)
				Expect(ctx.Err()).To(Equal(context.Canceled))
				Expect(fakeStepB.RunCallCount()).To(Equal(0))
			})

		})
	})

	Context("when steps fail", func() {
		Context("with normal error", func() {
			disasterA := errors.New("nope A")
			disasterB := errors.New("nope B")

			BeforeEach(func() {
				fakeStepA.RunReturns(disasterA)
				fakeStepB.RunReturns(disasterB)
			})

			Context("and fail fast is false", func() {
				BeforeEach(func() {
					step = InParallel(fakeSteps, 1, false)
				})
				It("lets all steps finish before exiting", func() {
					Expect(fakeStepA.RunCallCount()).To(Equal(1))
					Expect(fakeStepB.RunCallCount()).To(Equal(1))
				})
				It("exits with an error including the original message", func() {
					Expect(stepErr.Error()).To(ContainSubstring("nope A"))
					Expect(stepErr.Error()).To(ContainSubstring("nope B"))
				})
			})

			Context("and fail fast is true", func() {
				BeforeEach(func() {
					step = InParallel(fakeSteps, 1, true)
				})
				It("it cancels remaining steps", func() {
					Expect(fakeStepA.RunCallCount()).To(Equal(1))
					Expect(fakeStepB.RunCallCount()).To(Equal(0))
				})
				It("exits with an error including the message from the failed steps", func() {
					Expect(stepErr.Error()).To(ContainSubstring("nope A"))
					Expect(stepErr.Error()).NotTo(ContainSubstring("nope B"))
				})
			})
		})

		Context("with context canceled error", func() {
			// error might be wrapped. For example we pass context from in_parallel step
			// -> task step -> ... -> baggageclaim StreamOut() -> http request. When context
			// got canceled in in_parallel step, the http client sending the request will
			// wrap the context.Canceled error into Url.Error
			disasterB := fmt.Errorf("some thing failed by %w", context.Canceled)

			BeforeEach(func() {
				fakeStepB.RunReturns(disasterB)
			})

			It("exits with no error", func() {
				Expect(stepErr).ToNot(HaveOccurred())
			})
		})
	})

	Describe("Succeeded", func() {
		Context("when all steps are successful", func() {
			BeforeEach(func() {
				fakeStepA.SucceededReturns(true)
				fakeStepB.SucceededReturns(true)
			})

			It("yields true", func() {
				Expect(step.Succeeded()).To(BeTrue())
			})
		})

		Context("and some steps are not successful", func() {
			BeforeEach(func() {
				fakeStepA.SucceededReturns(true)
				fakeStepB.SucceededReturns(false)
			})

			It("yields false", func() {
				Expect(step.Succeeded()).To(BeFalse())
			})
		})

		Context("when no steps indicate success", func() {
			BeforeEach(func() {
				fakeStepA.SucceededReturns(false)
				fakeStepB.SucceededReturns(false)
			})

			It("returns false", func() {
				Expect(step.Succeeded()).To(BeFalse())
			})
		})

		Context("when there are no steps", func() {
			BeforeEach(func() {
				step = InParallelStep{}
			})

			It("returns true", func() {
				Expect(step.Succeeded()).To(BeTrue())
			})
		})
	})
})
