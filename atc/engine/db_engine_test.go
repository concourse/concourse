package engine_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	. "github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/engine/enginefakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DBEngine", func() {
	var (
		logger lager.Logger

		fakeEngineA *enginefakes.FakeEngine
		fakeEngineB *enginefakes.FakeEngine
		dbBuild     *dbfakes.FakeBuild

		dbEngine Engine
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeEngineA = new(enginefakes.FakeEngine)
		fakeEngineA.SchemaReturns("fake-schema-a")

		fakeEngineB = new(enginefakes.FakeEngine)
		fakeEngineB.SchemaReturns("fake-schema-b")

		dbBuild = new(dbfakes.FakeBuild)
		dbBuild.IDReturns(128)

		dbEngine = NewDBEngine(Engines{fakeEngineA, fakeEngineB})
	})

	Describe("CreateBuild", func() {
		var (
			plan atc.Plan

			createdBuild Build
			buildErr     error

			planFactory atc.PlanFactory
		)

		BeforeEach(func() {
			planFactory = atc.NewPlanFactory(123)

			plan = planFactory.NewPlan(atc.TaskPlan{
				Config: &atc.TaskConfig{
					RootfsURI: "some-image",

					Params: map[string]string{
						"FOO": "1",
						"BAR": "2",
					},

					Run: atc.TaskRunConfig{
						Path: "some-script",
						Args: []string{"arg1", "arg2"},
					},
				},
			})

			dbBuild.StartReturns(true, nil)
		})

		JustBeforeEach(func() {
			createdBuild, buildErr = dbEngine.CreateBuild(logger, dbBuild, plan)
		})

		Context("when creating the build succeeds", func() {
			It("succeeds", func() {
				Expect(buildErr).NotTo(HaveOccurred())
			})

			It("returns a build", func() {
				Expect(createdBuild).NotTo(BeNil())
			})

			It("starts the build in the database", func() {
				Expect(dbBuild.StartCallCount()).To(Equal(1))
			})

			Context("when the build fails to transition to started", func() {
				BeforeEach(func() {
					dbBuild.StartReturns(false, nil)
				})

				It("aborts the build", func() {
					Expect(dbBuild.FinishCallCount()).To(Equal(1))
					Expect(dbBuild.FinishArgsForCall(0)).To(Equal(db.BuildStatusAborted))
				})
			})
		})

		Context("when creating the build fails", func() {
			disaster := errors.New("failed")

			BeforeEach(func() {
				dbBuild.StartReturns(false, disaster)
			})

			It("returns the error", func() {
				Expect(buildErr).To(Equal(disaster))
			})
		})
	})

	Describe("LookupBuild", func() {
		var (
			foundBuild Build
			lookupErr  error
		)

		JustBeforeEach(func() {
			foundBuild, lookupErr = dbEngine.LookupBuild(logger, dbBuild)
		})

		It("succeeds", func() {
			Expect(lookupErr).NotTo(HaveOccurred())
		})

		It("returns a build", func() {
			Expect(foundBuild).NotTo(BeNil())
		})

		Describe("Abort", func() {
			var abortErr error

			BeforeEach(func() {
				dbBuild.ReloadReturns(true, nil)
			})

			JustBeforeEach(func() {
				abortErr = foundBuild.Abort(lagertest.NewTestLogger("test"))
			})

			Context("when acquiring the lock succeeds", func() {
				var fakeLock *lockfakes.FakeLock

				BeforeEach(func() {
					fakeLock = new(lockfakes.FakeLock)
					dbBuild.AcquireTrackingLockReturns(fakeLock, true, nil)
				})

				It("succeeds", func() {
					Expect(abortErr).NotTo(HaveOccurred())
				})

				It("marks the build as aborted", func() {
					Expect(dbBuild.MarkAsAbortedCallCount()).To(Equal(1))
				})
			})

			Context("when acquiring the lock fails", func() {
				BeforeEach(func() {
					dbBuild.AcquireTrackingLockReturns(nil, false, nil)
				})

				It("succeeds", func() {
					Expect(abortErr).NotTo(HaveOccurred())
				})

				It("marks the build as aborted", func() {
					Expect(dbBuild.MarkAsAbortedCallCount()).To(Equal(1))
				})
			})

			Context("when acquiring the lock errors", func() {
				BeforeEach(func() {
					dbBuild.AcquireTrackingLockReturns(nil, false, errors.New("bad bad bad"))
				})

				It("fails", func() {
					Expect(abortErr).To(HaveOccurred())
				})

				It("does not mark the build as aborted", func() {
					Expect(dbBuild.MarkAsAbortedCallCount()).To(Equal(0))
				})
			})
		})
	})

	Describe("Builds", func() {
		var build Build

		BeforeEach(func() {
			var err error
			build, err = dbEngine.LookupBuild(logger, dbBuild)
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("Abort", func() {
			var abortErr error

			JustBeforeEach(func() {
				abortErr = build.Abort(lagertest.NewTestLogger("test"))
			})

			Context("when acquiring the lock succeeds", func() {
				var fakeLock *lockfakes.FakeLock

				BeforeEach(func() {
					fakeLock = new(lockfakes.FakeLock)
					dbBuild.AcquireTrackingLockReturns(fakeLock, true, nil)
				})

				Context("when the build is active", func() {
					BeforeEach(func() {
						dbBuild.ReloadReturns(true, nil)
						dbBuild.SchemaReturns("fake-schema-b")

						dbBuild.MarkAsAbortedStub = func() error {
							Expect(dbBuild.AcquireTrackingLockCallCount()).To(Equal(1))

							_, interval := dbBuild.AcquireTrackingLockArgsForCall(0)
							Expect(interval).To(Equal(time.Minute))

							Expect(fakeLock.ReleaseCallCount()).To(BeZero())

							return nil
						}
					})

					Context("when the engine build exists", func() {
						var realBuild *enginefakes.FakeBuild

						BeforeEach(func() {
							dbBuild.ReloadReturns(true, nil)

							realBuild = new(enginefakes.FakeBuild)
							fakeEngineB.LookupBuildReturns(realBuild, nil)
						})

						Context("when aborting the db build succeeds", func() {
							BeforeEach(func() {
								dbBuild.MarkAsAbortedReturns(nil)
							})

							It("succeeds", func() {
								Expect(abortErr).NotTo(HaveOccurred())
							})

							It("releases the lock", func() {
								Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
							})

							It("aborts the build via the db", func() {
								Expect(dbBuild.MarkAsAbortedCallCount()).To(Equal(1))
							})

							It("aborts the real build", func() {
								Expect(realBuild.AbortCallCount()).To(Equal(1))
							})
						})

						Context("when aborting the db build fails", func() {
							disaster := errors.New("oh no!")

							BeforeEach(func() {
								dbBuild.MarkAsAbortedReturns(disaster)
							})

							It("returns the error", func() {
								Expect(abortErr).To(Equal(disaster))
							})

							It("does not abort the real build", func() {
								Expect(realBuild.AbortCallCount()).To(BeZero())
							})

							It("releases the lock", func() {
								Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
							})
						})

						Context("when aborting the real build fails", func() {
							disaster := errors.New("oh no!")

							BeforeEach(func() {
								realBuild.AbortReturns(disaster)
							})

							It("returns the error", func() {
								Expect(abortErr).To(Equal(disaster))
							})

							It("releases the lock", func() {
								Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
							})
						})
					})

					Context("when looking up the engine build fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							dbBuild.ReloadReturns(true, nil)
							fakeEngineB.LookupBuildReturns(nil, disaster)
						})

						It("returns the error", func() {
							Expect(abortErr).To(Equal(disaster))
						})

						It("releases the lock", func() {
							Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
						})
					})
				})

				Context("when the build is not yet active", func() {
					BeforeEach(func() {
						dbBuild.ReloadReturns(true, nil)
						dbBuild.SchemaReturns("")
					})

					It("succeeds", func() {
						Expect(abortErr).NotTo(HaveOccurred())
					})

					It("aborts the build in the db", func() {
						Expect(dbBuild.MarkAsAbortedCallCount()).To(Equal(1))
					})

					It("finishes the build in the db so that the aborted event is emitted", func() {
						Expect(dbBuild.FinishCallCount()).To(Equal(1))

						status := dbBuild.FinishArgsForCall(0)
						Expect(status).To(Equal(db.BuildStatusAborted))
					})

					It("releases the lock", func() {
						Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
					})
				})

				Context("when the build is no longer in the database", func() {
					BeforeEach(func() {
						dbBuild.ReloadReturns(false, nil)
					})

					It("succeeds", func() {
						Expect(abortErr).NotTo(HaveOccurred())
					})

					It("aborts the build in the db", func() {
						Expect(dbBuild.MarkAsAbortedCallCount()).To(Equal(1))
					})

					It("does not finish the build", func() {
						Expect(dbBuild.FinishCallCount()).To(Equal(0))
					})

					It("releases the lock", func() {
						Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
					})
				})
			})

			Context("when acquiring the lock errors", func() {
				BeforeEach(func() {
					dbBuild.AcquireTrackingLockReturns(nil, false, errors.New("bad bad bad"))
				})

				It("errors", func() {
					Expect(abortErr).To(HaveOccurred())
				})

				It("does not abort the build in the db", func() {
					Expect(dbBuild.MarkAsAbortedCallCount()).To(Equal(0))
				})
			})

			Context("when acquiring the lock fails", func() {
				BeforeEach(func() {
					dbBuild.AcquireTrackingLockReturns(nil, false, nil)
				})

				Context("when aborting the build in the db succeeds", func() {
					BeforeEach(func() {
						dbBuild.MarkAsAbortedReturns(nil)
					})

					It("succeeds", func() {
						Expect(abortErr).NotTo(HaveOccurred())
					})

					It("aborts the build in the db", func() {
						Expect(dbBuild.MarkAsAbortedCallCount()).To(Equal(1))
					})

					It("does not abort the real build", func() {
						Expect(dbBuild.ReloadCallCount()).To(BeZero())
						Expect(fakeEngineB.LookupBuildCallCount()).To(BeZero())
					})
				})

				Context("when aborting the build in the db fails", func() {
					disaster := errors.New("oh no!")

					BeforeEach(func() {
						dbBuild.MarkAsAbortedReturns(disaster)
					})

					It("fails", func() {
						Expect(abortErr).To(Equal(disaster))
					})
				})
			})
		})

		Describe("Resume", func() {
			var logger lager.Logger

			BeforeEach(func() {
				logger = lagertest.NewTestLogger("test")
			})

			JustBeforeEach(func() {
				build.Resume(logger)
			})

			Context("when acquiring the lock succeeds", func() {
				var fakeLock *lockfakes.FakeLock

				BeforeEach(func() {
					fakeLock = new(lockfakes.FakeLock)
					dbBuild.AcquireTrackingLockReturns(fakeLock, true, nil)
				})

				Context("when the build is active", func() {
					BeforeEach(func() {
						dbBuild.SchemaReturns("fake-schema-b")
						dbBuild.IsRunningReturns(true)
						dbBuild.ReloadReturns(true, nil)
					})

					Context("when the engine build exists", func() {
						var realBuild *enginefakes.FakeBuild

						BeforeEach(func() {
							dbBuild.ReloadReturns(true, nil)

							realBuild = new(enginefakes.FakeBuild)
							fakeEngineB.LookupBuildReturns(realBuild, nil)

							realBuild.ResumeStub = func(lager.Logger) {
								Expect(dbBuild.AcquireTrackingLockCallCount()).To(Equal(1))

								_, interval := dbBuild.AcquireTrackingLockArgsForCall(0)
								Expect(interval).To(Equal(time.Minute))

								Expect(fakeLock.ReleaseCallCount()).To(BeZero())
							}
						})

						Context("when builds are released", func() {
							BeforeEach(func() {
								readyToRelease := make(chan struct{})

								go func() {
									<-readyToRelease
									dbEngine.ReleaseAll(logger)
								}()

								relased := make(chan struct{})

								realBuild.ResumeStub = func(lager.Logger) {
									close(readyToRelease)
									<-relased
								}

								fakeEngineB.ReleaseAllStub = func(lager.Logger) {
									close(relased)
								}

								aborts := make(chan struct{})
								notifier := new(dbfakes.FakeNotifier)
								notifier.NotifyReturns(aborts)

								dbBuild.AbortNotifierReturns(notifier, nil)
							})

							It("releases build engine builds", func() {
								Expect(fakeEngineB.ReleaseAllCallCount()).To(Equal(1))
							})

							It("releases the lock", func() {
								Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
							})
						})

						Context("when listening for aborts succeeds", func() {
							var (
								notifier *dbfakes.FakeNotifier
								abort    chan<- struct{}
							)

							BeforeEach(func() {
								aborts := make(chan struct{})
								abort = aborts

								notifier = new(dbfakes.FakeNotifier)
								notifier.NotifyReturns(aborts)

								dbBuild.AbortNotifierReturns(notifier, nil)
							})

							It("listens for aborts", func() {
								Expect(dbBuild.AbortNotifierCallCount()).To(Equal(1))
							})

							It("resumes the build", func() {
								Expect(realBuild.ResumeCallCount()).To(Equal(1))
							})

							It("releases the lock", func() {
								Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
							})

							It("closes the notifier", func() {
								Expect(notifier.CloseCallCount()).To(Equal(1))
							})

							Context("when the build is aborted", func() {
								var errAborted = errors.New("aborted")

								BeforeEach(func() {
									aborted := make(chan error)

									realBuild.AbortStub = func(lager.Logger) error {
										aborted <- errAborted
										return nil
									}

									realBuild.ResumeStub = func(lager.Logger) {
										<-aborted
									}

									close(abort)
								})

								It("aborts the build", func() {
									Expect(realBuild.AbortCallCount()).To(Equal(1))
								})

								It("releases the lock", func() {
									Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
								})

								It("closes the notifier", func() {
									Expect(notifier.CloseCallCount()).To(Equal(1))
								})
							})
						})

						Context("when listening for aborts fails", func() {
							disaster := errors.New("oh no!")

							BeforeEach(func() {
								dbBuild.AbortNotifierReturns(nil, disaster)
							})

							It("does not resume the build", func() {
								Expect(realBuild.ResumeCallCount()).To(BeZero())
							})

							It("releases the lock", func() {
								Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
							})
						})
					})

					Context("when looking up the engine build fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							dbBuild.ReloadReturns(true, nil)
							fakeEngineB.LookupBuildReturns(nil, disaster)
						})

						It("releases the lock", func() {
							Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
						})

						It("marks the build as errored", func() {
							Expect(dbBuild.FinishWithErrorCallCount()).To(Equal(1))
							finishErr := dbBuild.FinishWithErrorArgsForCall(0)
							Expect(finishErr).To(Equal(disaster))
						})
					})
				})

				Context("when the build's schema is unknown", func() {
					BeforeEach(func() {
						dbBuild.ReloadReturns(true, nil)
						dbBuild.IsRunningReturns(true)
						dbBuild.SchemaReturns("bogus")
					})

					It("marks the build as errored", func() {
						Expect(dbBuild.FinishWithErrorCallCount()).To(Equal(1))
						finishErr := dbBuild.FinishWithErrorArgsForCall(0)
						Expect(finishErr).To(Equal(UnknownEngineError{Schema: "bogus"}))
					})
				})

				Context("when the build is not yet active", func() {
					BeforeEach(func() {
						dbBuild.ReloadReturns(true, nil)
						dbBuild.SchemaReturns("")
					})

					It("does not look up the build in the engine", func() {
						Expect(fakeEngineB.LookupBuildCallCount()).To(BeZero())
					})

					It("releases the lock", func() {
						Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
					})
				})

				Context("when the build has already finished", func() {
					BeforeEach(func() {
						dbBuild.ReloadReturns(true, nil)
						dbBuild.SchemaReturns("fake-schema-b")
						dbBuild.StatusReturns(db.BuildStatusSucceeded)
					})

					It("does not look up the build in the engine", func() {
						Expect(fakeEngineB.LookupBuildCallCount()).To(BeZero())
					})

					It("releases the lock", func() {
						Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
					})
				})

				Context("when the build is no longer in the database", func() {
					BeforeEach(func() {
						dbBuild.ReloadReturns(false, nil)
					})

					It("does not look up the build in the engine", func() {
						Expect(fakeEngineB.LookupBuildCallCount()).To(BeZero())
					})

					It("releases the lock", func() {
						Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
					})
				})
			})

			Context("when acquiring the lock fails", func() {
				BeforeEach(func() {
					dbBuild.AcquireTrackingLockReturns(nil, false, errors.New("no lock for you"))
				})

				It("does not look up the build", func() {
					Expect(dbBuild.ReloadCallCount()).To(BeZero())
					Expect(fakeEngineB.LookupBuildCallCount()).To(BeZero())
				})
			})
		})
	})
})
