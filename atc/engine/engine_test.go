package engine_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/builds"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	. "github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/engine/enginefakes"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/vars"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Engine", func() {
	var (
		fakeBuild          *dbfakes.FakeBuild
		fakeStepperFactory *enginefakes.FakeStepperFactory

		fakeGlobalCreds   *credsfakes.FakeSecrets
		fakeVarSourcePool *credsfakes.FakeVarSourcePool
	)

	BeforeEach(func() {
		fakeBuild = new(dbfakes.FakeBuild)
		fakeBuild.IDReturns(128)

		fakeStepperFactory = new(enginefakes.FakeStepperFactory)

		fakeGlobalCreds = new(credsfakes.FakeSecrets)
		fakeVarSourcePool = new(credsfakes.FakeVarSourcePool)
	})

	Describe("NewBuild", func() {
		var (
			build  builds.Runnable
			engine Engine
		)

		BeforeEach(func() {
			engine = NewEngine(fakeStepperFactory, fakeGlobalCreds, fakeVarSourcePool)
		})

		JustBeforeEach(func() {
			build = engine.NewBuild(fakeBuild)
		})

		It("returns a build", func() {
			Expect(build).NotTo(BeNil())
		})
	})

	Describe("Build", func() {
		var (
			build     builds.Runnable
			release   chan bool
			waitGroup *sync.WaitGroup
		)

		BeforeEach(func() {

			release = make(chan bool)
			trackedStates := new(sync.Map)
			waitGroup = new(sync.WaitGroup)

			build = NewBuild(
				fakeBuild,
				fakeStepperFactory,
				fakeGlobalCreds,
				fakeVarSourcePool,
				release,
				trackedStates,
				waitGroup,
			)
		})

		Describe("Run", func() {
			var (
				logger lager.Logger
				ctx    context.Context
			)

			BeforeEach(func() {
				logger = lagertest.NewTestLogger("test")
				ctx = context.Background()
			})

			JustBeforeEach(func() {
				build.Run(lagerctx.NewContext(ctx, logger))
			})

			Context("when acquiring the lock succeeds", func() {
				var fakeLock *lockfakes.FakeLock

				BeforeEach(func() {
					fakeLock = new(lockfakes.FakeLock)

					fakeBuild.AcquireTrackingLockReturns(fakeLock, true, nil)
				})

				Context("when the build is active", func() {
					BeforeEach(func() {
						fakeBuild.IsRunningReturns(true)
						fakeBuild.ReloadReturns(true, nil)
					})

					Context("when listening for aborts succeeds", func() {
						var abort chan struct{}
						var fakeNotifier *dbfakes.FakeNotifier

						BeforeEach(func() {
							abort = make(chan struct{})

							fakeNotifier = new(dbfakes.FakeNotifier)
							fakeNotifier.NotifyReturns(abort)

							fakeBuild.AbortNotifierReturns(fakeNotifier, nil)
						})

						Context("when converting the plan to a step succeeds", func() {
							var steppedPlans chan atc.Plan
							var fakeStep *execfakes.FakeStep

							BeforeEach(func() {
								fakeStep = new(execfakes.FakeStep)
								fakeBuild.PrivatePlanReturns(atc.Plan{
									ID: "build-plan",
									LoadVar: &atc.LoadVarPlan{
										Name: "some-var",
										File: "some-file.yml",
									},
								})

								steppedPlans = make(chan atc.Plan, 1)
								fakeStepperFactory.StepperForBuildReturns(func(plan atc.Plan) exec.Step {
									steppedPlans <- plan
									return fakeStep
								}, nil)
							})

							It("releases the lock", func() {
								waitGroup.Wait()
								Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
							})

							It("closes the notifier", func() {
								waitGroup.Wait()
								Expect(fakeNotifier.CloseCallCount()).To(Equal(1))
							})

							It("constructs a step from the build's plan", func() {
								plan := <-steppedPlans
								Expect(plan).ToNot(BeZero())
								Expect(plan).To(Equal(fakeBuild.PrivatePlan())) //XXX
							})

							Context("when getting the build vars succeeds", func() {
								var invokedState chan exec.RunState

								BeforeEach(func() {
									fakeBuild.VariablesReturns(vars.StaticVariables{"foo": "bar"}, nil)

									invokedState = make(chan exec.RunState, 1)
									fakeStep.RunStub = func(ctx context.Context, state exec.RunState) (bool, error) {
										invokedState <- state
										return true, nil
									}
								})

								It("runs the step with the build variables", func() {
									state := <-invokedState

									val, found, err := state.Get(vars.Reference{Path: "foo"})
									Expect(err).ToNot(HaveOccurred())
									Expect(found).To(BeTrue())
									Expect(val).To(Equal("bar"))
								})

								Context("when the build is released", func() {
									BeforeEach(func() {
										readyToRelease := make(chan bool)

										go func() {
											<-readyToRelease
											release <- true
										}()

										fakeStep.RunStub = func(context.Context, exec.RunState) (bool, error) {
											close(readyToRelease)
											<-time.After(time.Hour)
											return true, nil
										}
									})

									It("does not finish the build", func() {
										waitGroup.Wait()
										Expect(fakeBuild.FinishCallCount()).To(Equal(0))
									})
								})

								Context("when the build is aborted", func() {
									BeforeEach(func() {
										readyToAbort := make(chan bool)

										go func() {
											<-readyToAbort
											abort <- struct{}{}
										}()

										fakeStep.RunStub = func(context.Context, exec.RunState) (bool, error) {
											close(readyToAbort)
											<-time.After(time.Second)
											return true, nil
										}
									})

									It("cancels the context given to the step", func() {
										waitGroup.Wait()
										stepCtx, _ := fakeStep.RunArgsForCall(0)
										Expect(stepCtx.Done()).To(BeClosed())
									})
								})

								Context("when the build finishes successfully", func() {
									BeforeEach(func() {
										fakeStep.RunReturns(true, nil)
									})

									It("finishes the build", func() {
										waitGroup.Wait()
										Expect(fakeBuild.FinishCallCount()).To(Equal(1))
										Expect(fakeBuild.FinishArgsForCall(0)).To(Equal(db.BuildStatusSucceeded))
									})
								})

								Context("when the build finishes woefully", func() {
									BeforeEach(func() {
										fakeStep.RunReturns(false, nil)
									})

									It("finishes the build", func() {
										waitGroup.Wait()
										Expect(fakeBuild.FinishCallCount()).To(Equal(1))
										Expect(fakeBuild.FinishArgsForCall(0)).To(Equal(db.BuildStatusFailed))
									})
								})

								Context("when the build finishes with error", func() {
									Context("when the error is not retryable", func() {
										BeforeEach(func() {
											fakeStep.RunReturns(false, errors.New("nope"))
										})

										It("finishes the build", func() {
											waitGroup.Wait()
											Expect(fakeBuild.FinishCallCount()).To(Equal(1))
											Expect(fakeBuild.FinishArgsForCall(0)).To(Equal(db.BuildStatusErrored))
										})
									})

									Context("when the error is retryable", func() {
										BeforeEach(func() {
											fakeStep.RunReturns(false, exec.Retriable{Cause: errors.New("nope")})
										})

										Context("when this is a check build", func() {
											BeforeEach(func() {
												fakeBuild.NameReturns(db.CheckBuildName)
											})

											It("should not retry, thus finishes the build", func() {
												waitGroup.Wait()
												Expect(fakeBuild.FinishCallCount()).To(Equal(1))
												Expect(fakeBuild.FinishArgsForCall(0)).To(Equal(db.BuildStatusErrored))
											})
										})

										Context("when this is a normal build", func() {
											It("should retry, thus not finishe the build", func() {
												waitGroup.Wait()
												Expect(fakeBuild.FinishCallCount()).To(Equal(0))
											})
										})
									})
								})

								Context("when the build finishes with cancelled error", func() {
									BeforeEach(func() {
										fakeStep.RunReturns(false, context.Canceled)
									})

									It("finishes the build", func() {
										waitGroup.Wait()
										Expect(fakeBuild.FinishCallCount()).To(Equal(1))
										Expect(fakeBuild.FinishArgsForCall(0)).To(Equal(db.BuildStatusAborted))
									})
								})

								Context("when the build finishes with a wrapped cancelled error", func() {
									BeforeEach(func() {
										fakeStep.RunReturns(false, fmt.Errorf("but im not a wrapper: %w", context.Canceled))
									})

									It("finishes the build", func() {
										waitGroup.Wait()
										Expect(fakeBuild.FinishCallCount()).To(Equal(1))
										Expect(fakeBuild.FinishArgsForCall(0)).To(Equal(db.BuildStatusAborted))
									})
								})

								Context("when the build panics", func() {
									BeforeEach(func() {
										fakeStep.RunStub = func(context.Context, exec.RunState) (bool, error) {
											panic("something went wrong")
										}
									})

									It("finishes the build with error", func() {
										waitGroup.Wait()
										Expect(fakeBuild.FinishCallCount()).To(Equal(1))
										Expect(fakeBuild.FinishArgsForCall(0)).To(Equal(db.BuildStatusErrored))
									})
								})

								It("build.RunState should be called", func() {
									Expect(fakeBuild.RunStateIDCallCount()).To(Equal(2))
								})
							})

							Context("when getting the build vars fails", func() {
								BeforeEach(func() {
									fakeBuild.VariablesReturns(nil, errors.New("ruh roh"))
								})

								It("releases the lock", func() {
									Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
								})

								It("saves an error event", func() {
									Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
									Expect(fakeBuild.SaveEventArgsForCall(0).EventType()).To(Equal(event.EventTypeError))
								})
							})
						})

						Context("when converting the plan to a step fails", func() {
							BeforeEach(func() {
								fakeStepperFactory.StepperForBuildReturns(nil, errors.New("nope"))
							})

							It("releases the lock", func() {
								Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
							})

							It("saves an error event", func() {
								Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
								Expect(fakeBuild.SaveEventArgsForCall(0).EventType()).To(Equal(event.EventTypeError))
							})
						})
					})

					Context("when listening for aborts fails", func() {
						BeforeEach(func() {
							fakeBuild.AbortNotifierReturns(nil, errors.New("nope"))
						})

						It("releases the lock", func() {
							Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
						})
					})
				})

				Context("when the build is not yet active", func() {
					BeforeEach(func() {
						fakeBuild.ReloadReturns(true, nil)
					})

					It("does not build the step", func() {
						Expect(fakeStepperFactory.StepperForBuildCallCount()).To(BeZero())
					})

					It("releases the lock", func() {
						Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
					})
				})

				Context("when the build has already finished", func() {
					BeforeEach(func() {
						fakeBuild.ReloadReturns(true, nil)
						fakeBuild.StatusReturns(db.BuildStatusSucceeded)
					})

					It("does not build the step", func() {
						Expect(fakeStepperFactory.StepperForBuildCallCount()).To(BeZero())
					})

					It("releases the lock", func() {
						Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
					})
				})

				Context("when the build is no longer in the database", func() {
					BeforeEach(func() {
						fakeBuild.ReloadReturns(false, nil)
					})

					It("does not build the step", func() {
						Expect(fakeStepperFactory.StepperForBuildCallCount()).To(BeZero())
					})

					It("releases the lock", func() {
						Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
					})
				})
			})

			Context("when acquiring the lock fails", func() {
				BeforeEach(func() {
					fakeBuild.AcquireTrackingLockReturns(nil, false, errors.New("no lock for you"))
				})

				It("does not build the step", func() {
					Expect(fakeStepperFactory.StepperForBuildCallCount()).To(BeZero())
				})
			})
		})
	})
})
