package engine_test

import (
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
		fakeEngineA *fakes.FakeEngine
		fakeEngineB *fakes.FakeEngine
		fakeBuildDB *fakes.FakeBuildDB

		dbEngine Engine
	)

	BeforeEach(func() {
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
		)

		BeforeEach(func() {
			build = db.Build{
				ID:   128,
				Name: "some-build",
			}

			plan = atc.Plan{
				Task: &atc.TaskPlan{
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
				},
			}

			fakeBuildDB.StartBuildReturns(true, nil)
		})

		JustBeforeEach(func() {
			createdBuild, buildErr = dbEngine.CreateBuild(build, plan)
		})

		Context("when creating the build succeeds", func() {
			var fakeBuild *fakes.FakeBuild

			BeforeEach(func() {
				fakeBuild = new(fakes.FakeBuild)
				fakeBuild.MetadataReturns("some-metadata")

				fakeEngineA.CreateBuildReturns(fakeBuild, nil)
			})

			It("succeeds", func() {
				Ω(buildErr).ShouldNot(HaveOccurred())
			})

			It("returns a build", func() {
				Ω(createdBuild).ShouldNot(BeNil())
			})

			It("starts the build in the database", func() {
				Ω(fakeBuildDB.StartBuildCallCount()).Should(Equal(1))

				buildID, engine, metadata := fakeBuildDB.StartBuildArgsForCall(0)
				Ω(buildID).Should(Equal(128))
				Ω(engine).Should(Equal("fake-engine-a"))
				Ω(metadata).Should(Equal("some-metadata"))
			})

			Context("when the build fails to transition to started", func() {
				BeforeEach(func() {
					fakeBuildDB.StartBuildReturns(false, nil)
				})

				It("aborts the build", func() {
					Ω(fakeBuild.AbortCallCount()).Should(Equal(1))
				})
			})
		})

		Context("when creating the build fails", func() {
			disaster := errors.New("failed")

			BeforeEach(func() {
				fakeEngineA.CreateBuildReturns(nil, disaster)
			})

			It("returns the error", func() {
				Ω(buildErr).Should(Equal(disaster))
			})

			It("does not start the build", func() {
				Ω(fakeBuildDB.StartBuildCallCount()).Should(Equal(0))
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
			foundBuild, lookupErr = dbEngine.LookupBuild(build)
		})

		It("succeeds", func() {
			Ω(lookupErr).ShouldNot(HaveOccurred())
		})

		It("returns a build", func() {
			Ω(foundBuild).ShouldNot(BeNil())
		})

		Describe("Abort", func() {
			var abortErr error

			JustBeforeEach(func() {
				abortErr = foundBuild.Abort()
			})

			Context("when acquiring the lease succeeds", func() {
				var fakeLease *dbfakes.FakeLease

				BeforeEach(func() {
					fakeLease = new(dbfakes.FakeLease)
					fakeBuildDB.LeaseTrackReturns(fakeLease, true, nil)
				})

				It("succeeds", func() {
					Ω(abortErr).ShouldNot(HaveOccurred())
				})

				It("marks the build as aborted", func() {
					Ω(fakeBuildDB.AbortBuildCallCount()).Should(Equal(1))
					Ω(fakeBuildDB.AbortBuildArgsForCall(0)).Should(Equal(build.ID))
				})
			})

			Context("when acquiring the lease fails", func() {
				var fakeLease *dbfakes.FakeLease

				BeforeEach(func() {
					fakeLease = new(dbfakes.FakeLease)
					fakeBuildDB.LeaseTrackReturns(nil, false, nil)
				})

				It("succeeds", func() {
					Ω(abortErr).ShouldNot(HaveOccurred())
				})

				It("marks the build as aborted", func() {
					Ω(fakeBuildDB.AbortBuildCallCount()).Should(Equal(1))
					Ω(fakeBuildDB.AbortBuildArgsForCall(0)).Should(Equal(build.ID))
				})
			})

			Context("when acquiring the lease errors", func() {
				var fakeLease *dbfakes.FakeLease

				BeforeEach(func() {
					fakeLease = new(dbfakes.FakeLease)
					fakeBuildDB.LeaseTrackReturns(nil, false, errors.New("bad bad bad"))
				})

				It("fails", func() {
					Ω(abortErr).Should(HaveOccurred())
				})

				It("does not mark the build as aborted", func() {
					Ω(fakeBuildDB.AbortBuildCallCount()).Should(Equal(0))
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
			build, err = dbEngine.LookupBuild(model)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Describe("Abort", func() {
			var abortErr error

			JustBeforeEach(func() {
				abortErr = build.Abort()
			})

			Context("when acquiring the lock succeeds", func() {
				var fakeLease *dbfakes.FakeLease

				BeforeEach(func() {
					fakeLease = new(dbfakes.FakeLease)
					fakeBuildDB.LeaseTrackReturns(fakeLease, true, nil)
				})

				Context("when the build is active", func() {
					BeforeEach(func() {
						model.Engine = "fake-engine-b"

						fakeBuildDB.GetBuildReturns(model, nil)

						fakeBuildDB.AbortBuildStub = func(int) error {
							Ω(fakeBuildDB.LeaseTrackCallCount()).Should(Equal(1))

							lockedBuild, interval := fakeBuildDB.LeaseTrackArgsForCall(0)
							Ω(lockedBuild).Should(Equal(model.ID))
							Ω(interval).Should(Equal(time.Minute))

							Ω(fakeLease.BreakCallCount()).Should(BeZero())

							return nil
						}
					})

					Context("when the engine build exists", func() {
						var realBuild *fakes.FakeBuild

						BeforeEach(func() {
							fakeBuildDB.GetBuildReturns(model, nil)

							realBuild = new(fakes.FakeBuild)
							fakeEngineB.LookupBuildReturns(realBuild, nil)
						})

						Context("when aborting the db build succeeds", func() {
							BeforeEach(func() {
								fakeBuildDB.AbortBuildReturns(nil)
							})

							It("succeeds", func() {
								Ω(abortErr).ShouldNot(HaveOccurred())
							})

							It("breaks the lease", func() {
								Ω(fakeLease.BreakCallCount()).Should(Equal(1))
							})

							It("aborts the build via the db", func() {
								Ω(fakeBuildDB.AbortBuildCallCount()).Should(Equal(1))

								buildID := fakeBuildDB.AbortBuildArgsForCall(0)
								Ω(buildID).Should(Equal(model.ID))
							})

							It("aborts the real build", func() {
								Ω(realBuild.AbortCallCount()).Should(Equal(1))
							})
						})

						Context("when aborting the db build fails", func() {
							disaster := errors.New("oh no!")

							BeforeEach(func() {
								fakeBuildDB.AbortBuildReturns(disaster)
							})

							It("returns the error", func() {
								Ω(abortErr).Should(Equal(disaster))
							})

							It("does not abort the real build", func() {
								Ω(realBuild.AbortCallCount()).Should(BeZero())
							})

							It("releases the lease", func() {
								Ω(fakeLease.BreakCallCount()).Should(Equal(1))
							})
						})

						Context("when aborting the real build fails", func() {
							disaster := errors.New("oh no!")

							BeforeEach(func() {
								realBuild.AbortReturns(disaster)
							})

							It("returns the error", func() {
								Ω(abortErr).Should(Equal(disaster))
							})

							It("releases the lock", func() {
								Ω(fakeLease.BreakCallCount()).Should(Equal(1))
							})
						})
					})

					Context("when looking up the engine build fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeBuildDB.GetBuildReturns(model, nil)
							fakeEngineB.LookupBuildReturns(nil, disaster)
						})

						It("returns the error", func() {
							Ω(abortErr).Should(Equal(disaster))
						})

						It("breaks the lease", func() {
							Ω(fakeLease.BreakCallCount()).Should(Equal(1))
						})
					})
				})

				Context("when the build is not yet active", func() {
					BeforeEach(func() {
						model.Engine = ""
						fakeBuildDB.GetBuildReturns(model, nil)
					})

					It("succeeds", func() {
						Ω(abortErr).ShouldNot(HaveOccurred())
					})

					It("aborts the build in the db", func() {
						Ω(fakeBuildDB.AbortBuildCallCount()).Should(Equal(1))

						buildID := fakeBuildDB.AbortBuildArgsForCall(0)
						Ω(buildID).Should(Equal(model.ID))
					})

					It("finishes the build in the db so that the aborted event is emitted", func() {
						Ω(fakeBuildDB.FinishBuildCallCount()).Should(Equal(1))

						buildID, status := fakeBuildDB.FinishBuildArgsForCall(0)
						Ω(buildID).Should(Equal(model.ID))
						Ω(status).Should(Equal(db.StatusAborted))
					})

					It("breaks the lease", func() {
						Ω(fakeLease.BreakCallCount()).Should(Equal(1))
					})
				})
			})

			Context("when acquiring the lock errors", func() {
				BeforeEach(func() {
					fakeBuildDB.LeaseTrackReturns(nil, false, errors.New("bad bad bad"))
				})

				It("errors", func() {
					Ω(abortErr).Should(HaveOccurred())
				})

				It("does not abort the build in the db", func() {
					Ω(fakeBuildDB.AbortBuildCallCount()).Should(Equal(0))
				})
			})

			Context("when acquiring the lock fails", func() {
				BeforeEach(func() {
					fakeBuildDB.LeaseTrackReturns(nil, false, nil)
				})

				Context("when aborting the build in the db succeeds", func() {
					BeforeEach(func() {
						fakeBuildDB.AbortBuildReturns(nil)
					})

					It("succeeds", func() {
						Ω(abortErr).ShouldNot(HaveOccurred())
					})

					It("aborts the build in the db", func() {
						Ω(fakeBuildDB.AbortBuildCallCount()).Should(Equal(1))
						Ω(fakeBuildDB.AbortBuildArgsForCall(0)).Should(Equal(model.ID))
					})

					It("does not abort the real build", func() {
						Ω(fakeBuildDB.GetBuildCallCount()).Should(BeZero())
						Ω(fakeEngineB.LookupBuildCallCount()).Should(BeZero())
					})
				})

				Context("when aborting the build in the db fails", func() {
					disaster := errors.New("oh no!")

					BeforeEach(func() {
						fakeBuildDB.AbortBuildReturns(disaster)
					})

					It("fails", func() {
						Ω(abortErr).Should(Equal(disaster))
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
					fakeBuildDB.LeaseTrackReturns(fakeLease, true, nil)
				})

				Context("when the build is active", func() {
					BeforeEach(func() {
						model.Engine = "fake-engine-b"
						fakeBuildDB.GetBuildReturns(model, nil)
					})

					Context("when the engine build exists", func() {
						var realBuild *fakes.FakeBuild

						BeforeEach(func() {
							fakeBuildDB.GetBuildReturns(model, nil)

							realBuild = new(fakes.FakeBuild)
							fakeEngineB.LookupBuildReturns(realBuild, nil)

							realBuild.ResumeStub = func(lager.Logger) {
								Ω(fakeBuildDB.LeaseTrackCallCount()).Should(Equal(1))

								lockedBuild, interval := fakeBuildDB.LeaseTrackArgsForCall(0)
								Ω(lockedBuild).Should(Equal(model.ID))
								Ω(interval).Should(Equal(time.Minute))

								Ω(fakeLease.BreakCallCount()).Should(BeZero())
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
								Ω(fakeBuildDB.AbortNotifierCallCount()).Should(Equal(1))
								Ω(fakeBuildDB.AbortNotifierArgsForCall(0)).Should(Equal(model.ID))
							})

							It("resumes the build", func() {
								Ω(realBuild.ResumeCallCount()).Should(Equal(1))
							})

							It("breaks the lease", func() {
								Ω(fakeLease.BreakCallCount()).Should(Equal(1))
							})

							It("closes the notifier", func() {
								Ω(notifier.CloseCallCount()).Should(Equal(1))
							})

							Context("when the build is aborted", func() {
								var errAborted = errors.New("aborted")

								BeforeEach(func() {
									aborted := make(chan error)

									realBuild.AbortStub = func() error {
										aborted <- errAborted
										return nil
									}

									realBuild.ResumeStub = func(lager.Logger) {
										<-aborted
									}

									close(abort)
								})

								It("aborts the build", func() {
									Ω(realBuild.AbortCallCount()).Should(Equal(1))
								})

								It("breaks the lease", func() {
									Ω(fakeLease.BreakCallCount()).Should(Equal(1))
								})

								It("closes the notifier", func() {
									Ω(notifier.CloseCallCount()).Should(Equal(1))
								})
							})
						})

						Context("when listening for aborts fails", func() {
							disaster := errors.New("oh no!")

							BeforeEach(func() {
								fakeBuildDB.AbortNotifierReturns(nil, disaster)
							})

							It("does not resume the build", func() {
								Ω(realBuild.ResumeCallCount()).Should(BeZero())
							})

							It("breaks the lease", func() {
								Ω(fakeLease.BreakCallCount()).Should(Equal(1))
							})
						})
					})

					Context("when looking up the engine build fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeBuildDB.GetBuildReturns(model, nil)
							fakeEngineB.LookupBuildReturns(nil, disaster)
						})

						It("breaks the lease", func() {
							Ω(fakeLease.BreakCallCount()).Should(Equal(1))
						})

						It("marks the build as errored", func() {
							Ω(fakeBuildDB.FinishBuildCallCount()).Should(Equal(1))
							buildID, buildStatus := fakeBuildDB.FinishBuildArgsForCall(0)
							Ω(buildID).Should(Equal(model.ID))
							Ω(buildStatus).Should(Equal(db.StatusErrored))
						})
					})
				})

				Context("when the build's engine is unknown", func() {
					BeforeEach(func() {
						model.Engine = "bogus"
						fakeBuildDB.GetBuildReturns(model, nil)
					})

					It("marks the build as errored", func() {
						Ω(fakeBuildDB.FinishBuildCallCount()).Should(Equal(1))
						buildID, buildStatus := fakeBuildDB.FinishBuildArgsForCall(0)
						Ω(buildID).Should(Equal(model.ID))
						Ω(buildStatus).Should(Equal(db.StatusErrored))
					})
				})

				Context("when the build is not yet active", func() {
					BeforeEach(func() {
						model.Engine = ""
						fakeBuildDB.GetBuildReturns(model, nil)
					})

					It("does not look up the build in the engine", func() {
						Ω(fakeEngineB.LookupBuildCallCount()).Should(BeZero())
					})

					It("breaks the lease", func() {
						Ω(fakeLease.BreakCallCount()).Should(Equal(1))
					})
				})

				Context("when the build has already finished", func() {
					BeforeEach(func() {
						model.Engine = "fake-engine-b"
						model.Status = db.StatusSucceeded
						fakeBuildDB.GetBuildReturns(model, nil)
					})

					It("does not look up the build in the engine", func() {
						Ω(fakeEngineB.LookupBuildCallCount()).Should(BeZero())
					})

					It("breaks the lease", func() {
						Ω(fakeLease.BreakCallCount()).Should(Equal(1))
					})
				})
			})

			Context("when acquiring the lock fails", func() {
				BeforeEach(func() {
					fakeBuildDB.LeaseTrackReturns(nil, false, errors.New("no lease for you"))
				})

				It("does not look up the build", func() {
					Ω(fakeBuildDB.GetBuildCallCount()).Should(BeZero())
					Ω(fakeEngineB.LookupBuildCallCount()).Should(BeZero())
				})
			})
		})
	})
})
