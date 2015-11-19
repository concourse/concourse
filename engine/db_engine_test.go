package engine_test

import (
	"encoding/json"
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	. "github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/fakes"
)

var _ = Describe("DBEngine", func() {
	var (
		logger lager.Logger

		fakeEngineA *fakes.FakeEngine
		fakeEngineB *fakes.FakeEngine
		fakeBuildDB *fakes.FakeBuildDB

		dbEngine Engine
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeEngineA = new(fakes.FakeEngine)
		fakeEngineA.NameReturns("fake-engine-a")

		fakeEngineB = new(fakes.FakeEngine)
		fakeEngineB.NameReturns("fake-engine-b")

		fakeBuildDB = new(fakes.FakeBuildDB)

		dbEngine = NewDBEngine(Engines{fakeEngineA, fakeEngineB}, fakeBuildDB)
	})

	Describe("CreateBuild", func() {
		var (
			build db.Build
			plan  atc.Plan

			createdBuild Build
			buildErr     error

			planFactory atc.PlanFactory
		)

		BeforeEach(func() {
			planFactory = atc.NewPlanFactory(123)

			build = db.Build{
				ID:   128,
				Name: "some-build",
			}

			plan = planFactory.NewPlan(atc.TaskPlan{
				Config: &atc.TaskConfig{
					Image: "some-image",

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

			fakeBuildDB.StartBuildReturns(true, nil)
		})

		JustBeforeEach(func() {
			createdBuild, buildErr = dbEngine.CreateBuild(logger, build, plan)
		})

		Context("when creating the build succeeds", func() {
			var fakeBuild *fakes.FakeBuild

			BeforeEach(func() {
				fakeBuild = new(fakes.FakeBuild)
				fakeBuild.MetadataReturns("some-metadata")

				fakeEngineA.CreateBuildReturns(fakeBuild, nil)
			})

			It("succeeds", func() {
				Expect(buildErr).NotTo(HaveOccurred())
			})

			It("returns a build", func() {
				Expect(createdBuild).NotTo(BeNil())
			})

			It("starts the build in the database", func() {
				Expect(fakeBuildDB.StartBuildCallCount()).To(Equal(1))

				buildID, engine, metadata := fakeBuildDB.StartBuildArgsForCall(0)
				Expect(buildID).To(Equal(128))
				Expect(engine).To(Equal("fake-engine-a"))
				Expect(metadata).To(Equal("some-metadata"))
			})

			Context("when the build fails to transition to started", func() {
				BeforeEach(func() {
					fakeBuildDB.StartBuildReturns(false, nil)
				})

				It("aborts the build", func() {
					Expect(fakeBuild.AbortCallCount()).To(Equal(1))
				})
			})
		})

		Context("when creating the build fails", func() {
			disaster := errors.New("failed")

			BeforeEach(func() {
				fakeEngineA.CreateBuildReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(buildErr).To(Equal(disaster))
			})

			It("does not start the build", func() {
				Expect(fakeBuildDB.StartBuildCallCount()).To(Equal(0))
			})
		})
	})

	Describe("LookupBuild", func() {
		var (
			build db.Build

			foundBuild Build
			lookupErr  error
		)

		BeforeEach(func() {
			build = db.Build{
				ID:   128,
				Name: "some-build",
			}
		})

		JustBeforeEach(func() {
			foundBuild, lookupErr = dbEngine.LookupBuild(logger, build)
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
				fakeBuildDB.GetBuildReturns(build, true, nil)
			})

			JustBeforeEach(func() {
				abortErr = foundBuild.Abort(lagertest.NewTestLogger("test"))
			})

			Context("when acquiring the lease succeeds", func() {
				var fakeLease *dbfakes.FakeLease

				BeforeEach(func() {
					fakeLease = new(dbfakes.FakeLease)
					fakeBuildDB.LeaseBuildTrackingReturns(fakeLease, true, nil)
				})

				It("succeeds", func() {
					Expect(abortErr).NotTo(HaveOccurred())
				})

				It("marks the build as aborted", func() {
					Expect(fakeBuildDB.AbortBuildCallCount()).To(Equal(1))
					Expect(fakeBuildDB.AbortBuildArgsForCall(0)).To(Equal(build.ID))
				})
			})

			Context("when acquiring the lease fails", func() {
				var fakeLease *dbfakes.FakeLease

				BeforeEach(func() {
					fakeLease = new(dbfakes.FakeLease)
					fakeBuildDB.LeaseBuildTrackingReturns(nil, false, nil)
				})

				It("succeeds", func() {
					Expect(abortErr).NotTo(HaveOccurred())
				})

				It("marks the build as aborted", func() {
					Expect(fakeBuildDB.AbortBuildCallCount()).To(Equal(1))
					Expect(fakeBuildDB.AbortBuildArgsForCall(0)).To(Equal(build.ID))
				})
			})

			Context("when acquiring the lease errors", func() {
				var fakeLease *dbfakes.FakeLease

				BeforeEach(func() {
					fakeLease = new(dbfakes.FakeLease)
					fakeBuildDB.LeaseBuildTrackingReturns(nil, false, errors.New("bad bad bad"))
				})

				It("fails", func() {
					Expect(abortErr).To(HaveOccurred())
				})

				It("does not mark the build as aborted", func() {
					Expect(fakeBuildDB.AbortBuildCallCount()).To(Equal(0))
				})
			})
		})
	})

	Describe("Builds", func() {
		var (
			build Build
			model db.Build
		)

		BeforeEach(func() {
			model = db.Build{
				ID: 128,

				Status: db.StatusStarted,
				Engine: "fake-engine-b",
			}

			var err error
			build, err = dbEngine.LookupBuild(logger, model)
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("Abort", func() {
			var abortErr error

			JustBeforeEach(func() {
				abortErr = build.Abort(lagertest.NewTestLogger("test"))
			})

			Context("when acquiring the lock succeeds", func() {
				var fakeLease *dbfakes.FakeLease

				BeforeEach(func() {
					fakeLease = new(dbfakes.FakeLease)
					fakeBuildDB.LeaseBuildTrackingReturns(fakeLease, true, nil)
				})

				Context("when the build is active", func() {
					BeforeEach(func() {
						model.Engine = "fake-engine-b"

						fakeBuildDB.GetBuildReturns(model, true, nil)

						fakeBuildDB.AbortBuildStub = func(int) error {
							Expect(fakeBuildDB.LeaseBuildTrackingCallCount()).To(Equal(1))

							lockedBuild, interval := fakeBuildDB.LeaseBuildTrackingArgsForCall(0)
							Expect(lockedBuild).To(Equal(model.ID))
							Expect(interval).To(Equal(10 * time.Second))

							Expect(fakeLease.BreakCallCount()).To(BeZero())

							return nil
						}
					})

					Context("when the engine build exists", func() {
						var realBuild *fakes.FakeBuild

						BeforeEach(func() {
							fakeBuildDB.GetBuildReturns(model, true, nil)

							realBuild = new(fakes.FakeBuild)
							fakeEngineB.LookupBuildReturns(realBuild, nil)
						})

						Context("when aborting the db build succeeds", func() {
							BeforeEach(func() {
								fakeBuildDB.AbortBuildReturns(nil)
							})

							It("succeeds", func() {
								Expect(abortErr).NotTo(HaveOccurred())
							})

							It("breaks the lease", func() {
								Expect(fakeLease.BreakCallCount()).To(Equal(1))
							})

							It("aborts the build via the db", func() {
								Expect(fakeBuildDB.AbortBuildCallCount()).To(Equal(1))

								buildID := fakeBuildDB.AbortBuildArgsForCall(0)
								Expect(buildID).To(Equal(model.ID))
							})

							It("aborts the real build", func() {
								Expect(realBuild.AbortCallCount()).To(Equal(1))
							})
						})

						Context("when aborting the db build fails", func() {
							disaster := errors.New("oh no!")

							BeforeEach(func() {
								fakeBuildDB.AbortBuildReturns(disaster)
							})

							It("returns the error", func() {
								Expect(abortErr).To(Equal(disaster))
							})

							It("does not abort the real build", func() {
								Expect(realBuild.AbortCallCount()).To(BeZero())
							})

							It("releases the lease", func() {
								Expect(fakeLease.BreakCallCount()).To(Equal(1))
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
								Expect(fakeLease.BreakCallCount()).To(Equal(1))
							})
						})
					})

					Context("when looking up the engine build fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeBuildDB.GetBuildReturns(model, true, nil)
							fakeEngineB.LookupBuildReturns(nil, disaster)
						})

						It("returns the error", func() {
							Expect(abortErr).To(Equal(disaster))
						})

						It("breaks the lease", func() {
							Expect(fakeLease.BreakCallCount()).To(Equal(1))
						})
					})
				})

				Context("when the build is not yet active", func() {
					BeforeEach(func() {
						model.Engine = ""
						fakeBuildDB.GetBuildReturns(model, true, nil)
					})

					It("succeeds", func() {
						Expect(abortErr).NotTo(HaveOccurred())
					})

					It("aborts the build in the db", func() {
						Expect(fakeBuildDB.AbortBuildCallCount()).To(Equal(1))

						buildID := fakeBuildDB.AbortBuildArgsForCall(0)
						Expect(buildID).To(Equal(model.ID))
					})

					It("finishes the build in the db so that the aborted event is emitted", func() {
						Expect(fakeBuildDB.FinishBuildCallCount()).To(Equal(1))

						buildID, status := fakeBuildDB.FinishBuildArgsForCall(0)
						Expect(buildID).To(Equal(model.ID))
						Expect(status).To(Equal(db.StatusAborted))
					})

					It("breaks the lease", func() {
						Expect(fakeLease.BreakCallCount()).To(Equal(1))
					})
				})

				Context("when the build is no longer in the database", func() {
					BeforeEach(func() {
						fakeBuildDB.GetBuildReturns(db.Build{}, false, nil)
					})

					It("succeeds", func() {
						Expect(abortErr).NotTo(HaveOccurred())
					})

					It("aborts the build in the db", func() {
						Expect(fakeBuildDB.AbortBuildCallCount()).To(Equal(1))

						buildID := fakeBuildDB.AbortBuildArgsForCall(0)
						Expect(buildID).To(Equal(model.ID))
					})

					It("does not finish the build", func() {
						Expect(fakeBuildDB.FinishBuildCallCount()).To(Equal(0))
					})

					It("breaks the lease", func() {
						Expect(fakeLease.BreakCallCount()).To(Equal(1))
					})
				})
			})

			Context("when acquiring the lock errors", func() {
				BeforeEach(func() {
					fakeBuildDB.LeaseBuildTrackingReturns(nil, false, errors.New("bad bad bad"))
				})

				It("errors", func() {
					Expect(abortErr).To(HaveOccurred())
				})

				It("does not abort the build in the db", func() {
					Expect(fakeBuildDB.AbortBuildCallCount()).To(Equal(0))
				})
			})

			Context("when acquiring the lock fails", func() {
				BeforeEach(func() {
					fakeBuildDB.LeaseBuildTrackingReturns(nil, false, nil)
				})

				Context("when aborting the build in the db succeeds", func() {
					BeforeEach(func() {
						fakeBuildDB.AbortBuildReturns(nil)
					})

					It("succeeds", func() {
						Expect(abortErr).NotTo(HaveOccurred())
					})

					It("aborts the build in the db", func() {
						Expect(fakeBuildDB.AbortBuildCallCount()).To(Equal(1))
						Expect(fakeBuildDB.AbortBuildArgsForCall(0)).To(Equal(model.ID))
					})

					It("does not abort the real build", func() {
						Expect(fakeBuildDB.GetBuildCallCount()).To(BeZero())
						Expect(fakeEngineB.LookupBuildCallCount()).To(BeZero())
					})
				})

				Context("when aborting the build in the db fails", func() {
					disaster := errors.New("oh no!")

					BeforeEach(func() {
						fakeBuildDB.AbortBuildReturns(disaster)
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
				var fakeLease *dbfakes.FakeLease

				BeforeEach(func() {
					fakeLease = new(dbfakes.FakeLease)
					fakeBuildDB.LeaseBuildTrackingReturns(fakeLease, true, nil)
				})

				Context("when the build is active", func() {
					BeforeEach(func() {
						model.Engine = "fake-engine-b"
						fakeBuildDB.GetBuildReturns(model, true, nil)
					})

					Context("when the engine build exists", func() {
						var realBuild *fakes.FakeBuild

						BeforeEach(func() {
							fakeBuildDB.GetBuildReturns(model, true, nil)

							realBuild = new(fakes.FakeBuild)
							fakeEngineB.LookupBuildReturns(realBuild, nil)

							realBuild.ResumeStub = func(lager.Logger) {
								Expect(fakeBuildDB.LeaseBuildTrackingCallCount()).To(Equal(1))

								lockedBuild, interval := fakeBuildDB.LeaseBuildTrackingArgsForCall(0)
								Expect(lockedBuild).To(Equal(model.ID))
								Expect(interval).To(Equal(10 * time.Second))

								Expect(fakeLease.BreakCallCount()).To(BeZero())
							}
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

								fakeBuildDB.AbortNotifierReturns(notifier, nil)
							})

							It("listens for aborts", func() {
								Expect(fakeBuildDB.AbortNotifierCallCount()).To(Equal(1))
								Expect(fakeBuildDB.AbortNotifierArgsForCall(0)).To(Equal(model.ID))
							})

							It("resumes the build", func() {
								Expect(realBuild.ResumeCallCount()).To(Equal(1))
							})

							It("breaks the lease", func() {
								Expect(fakeLease.BreakCallCount()).To(Equal(1))
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

								It("breaks the lease", func() {
									Expect(fakeLease.BreakCallCount()).To(Equal(1))
								})

								It("closes the notifier", func() {
									Expect(notifier.CloseCallCount()).To(Equal(1))
								})
							})
						})

						Context("when listening for aborts fails", func() {
							disaster := errors.New("oh no!")

							BeforeEach(func() {
								fakeBuildDB.AbortNotifierReturns(nil, disaster)
							})

							It("does not resume the build", func() {
								Expect(realBuild.ResumeCallCount()).To(BeZero())
							})

							It("breaks the lease", func() {
								Expect(fakeLease.BreakCallCount()).To(Equal(1))
							})
						})
					})

					Context("when looking up the engine build fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeBuildDB.GetBuildReturns(model, true, nil)
							fakeEngineB.LookupBuildReturns(nil, disaster)
						})

						It("breaks the lease", func() {
							Expect(fakeLease.BreakCallCount()).To(Equal(1))
						})

						It("marks the build as errored", func() {
							Expect(fakeBuildDB.FinishBuildCallCount()).To(Equal(1))
							buildID, buildStatus := fakeBuildDB.FinishBuildArgsForCall(0)
							Expect(buildID).To(Equal(model.ID))
							Expect(buildStatus).To(Equal(db.StatusErrored))
						})
					})
				})

				Context("when the build's engine is unknown", func() {
					BeforeEach(func() {
						model.Engine = "bogus"
						fakeBuildDB.GetBuildReturns(model, true, nil)
					})

					It("marks the build as errored", func() {
						Expect(fakeBuildDB.FinishBuildCallCount()).To(Equal(1))
						buildID, buildStatus := fakeBuildDB.FinishBuildArgsForCall(0)
						Expect(buildID).To(Equal(model.ID))
						Expect(buildStatus).To(Equal(db.StatusErrored))
					})
				})

				Context("when the build is not yet active", func() {
					BeforeEach(func() {
						model.Engine = ""
						fakeBuildDB.GetBuildReturns(model, true, nil)
					})

					It("does not look up the build in the engine", func() {
						Expect(fakeEngineB.LookupBuildCallCount()).To(BeZero())
					})

					It("breaks the lease", func() {
						Expect(fakeLease.BreakCallCount()).To(Equal(1))
					})
				})

				Context("when the build has already finished", func() {
					BeforeEach(func() {
						model.Engine = "fake-engine-b"
						model.Status = db.StatusSucceeded
						fakeBuildDB.GetBuildReturns(model, true, nil)
					})

					It("does not look up the build in the engine", func() {
						Expect(fakeEngineB.LookupBuildCallCount()).To(BeZero())
					})

					It("breaks the lease", func() {
						Expect(fakeLease.BreakCallCount()).To(Equal(1))
					})
				})

				Context("when the build is no longer in the database", func() {
					BeforeEach(func() {
						fakeBuildDB.GetBuildReturns(db.Build{}, false, nil)
					})

					It("does not look up the build in the engine", func() {
						Expect(fakeEngineB.LookupBuildCallCount()).To(BeZero())
					})

					It("breaks the lease", func() {
						Expect(fakeLease.BreakCallCount()).To(Equal(1))
					})
				})
			})

			Context("when acquiring the lock fails", func() {
				BeforeEach(func() {
					fakeBuildDB.LeaseBuildTrackingReturns(nil, false, errors.New("no lease for you"))
				})

				It("does not look up the build", func() {
					Expect(fakeBuildDB.GetBuildCallCount()).To(BeZero())
					Expect(fakeEngineB.LookupBuildCallCount()).To(BeZero())
				})
			})
		})

		Describe("PublicPlan", func() {
			var logger lager.Logger

			var publicPlan atc.PublicBuildPlan
			var planFound bool
			var publicPlanErr error

			BeforeEach(func() {
				logger = lagertest.NewTestLogger("test")
			})

			JustBeforeEach(func() {
				publicPlan, planFound, publicPlanErr = build.PublicPlan(logger)
			})

			var fakeLease *dbfakes.FakeLease

			BeforeEach(func() {
				fakeLease = new(dbfakes.FakeLease)
				fakeBuildDB.LeaseBuildTrackingReturns(fakeLease, true, nil)
			})

			Context("when the build is active", func() {
				BeforeEach(func() {
					model.Engine = "fake-engine-b"
					fakeBuildDB.GetBuildReturns(model, true, nil)
				})

				Context("when the engine build exists", func() {
					var realBuild *fakes.FakeBuild

					BeforeEach(func() {
						fakeBuildDB.GetBuildReturns(model, true, nil)

						realBuild = new(fakes.FakeBuild)
						fakeEngineB.LookupBuildReturns(realBuild, nil)
					})

					Context("when getting the plan via the engine succeeds", func() {
						BeforeEach(func() {
							var plan json.RawMessage = []byte("lol")

							realBuild.PublicPlanReturns(atc.PublicBuildPlan{
								Schema: "some-schema",
								Plan:   &plan,
							}, true, nil)
						})

						It("succeeds", func() {
							Expect(publicPlanErr).ToNot(HaveOccurred())
							Expect(planFound).To(BeTrue())
						})

						It("returns the public plan from the engine", func() {
							var plan json.RawMessage = []byte("lol")

							Expect(publicPlan).To(Equal(atc.PublicBuildPlan{
								Schema: "some-schema",
								Plan:   &plan,
							}))
						})
					})

					Context("when getting the plan via the engine fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							realBuild.PublicPlanReturns(atc.PublicBuildPlan{}, false, disaster)
						})

						It("returns the error", func() {
							Expect(publicPlanErr).To(Equal(disaster))
							Expect(planFound).To(BeFalse())
						})
					})
				})

				Context("when looking up the engine build fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeBuildDB.GetBuildReturns(model, true, nil)
						fakeEngineB.LookupBuildReturns(nil, disaster)
					})

					It("returns the error", func() {
						Expect(publicPlanErr).To(Equal(disaster))
						Expect(planFound).To(BeFalse())
					})
				})
			})

			Context("when the build's engine is unknown", func() {
				BeforeEach(func() {
					model.Engine = "bogus"
					fakeBuildDB.GetBuildReturns(model, true, nil)
				})

				It("returns an UnknownEngineError", func() {
					Expect(publicPlanErr).To(Equal(UnknownEngineError{"bogus"}))
					Expect(planFound).To(BeFalse())
				})
			})

			Context("when the build is not yet active", func() {
				BeforeEach(func() {
					model.Engine = ""
					fakeBuildDB.GetBuildReturns(model, true, nil)
				})

				It("does not look up the build in the engine", func() {
					Expect(fakeEngineB.LookupBuildCallCount()).To(BeZero())
				})

				It("does not find the plan", func() {
					Expect(planFound).To(BeFalse())
				})
			})

			Context("when the build is no longer in the database", func() {
				BeforeEach(func() {
					fakeBuildDB.GetBuildReturns(db.Build{}, false, nil)
				})

				It("does not look up the build in the engine", func() {
					Expect(fakeEngineB.LookupBuildCallCount()).To(BeZero())
				})

				It("does not find the plan", func() {
					Expect(planFound).To(BeFalse())
				})
			})
		})
	})
})
