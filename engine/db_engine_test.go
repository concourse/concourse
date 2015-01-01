package engine_test

import (
	"bytes"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	. "github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/fakes"

	garden "github.com/cloudfoundry-incubator/garden/api"
	gardenfakes "github.com/cloudfoundry-incubator/garden/api/fakes"
)

var _ = Describe("DBEngine", func() {
	var (
		fakeEngine  *fakes.FakeEngine
		fakeBuildDB *fakes.FakeBuildDB
		fakeLocker  *fakes.FakeBuildLocker

		dbEngine Engine
	)

	BeforeEach(func() {
		fakeEngine = new(fakes.FakeEngine)
		fakeEngine.NameReturns("fake-engine")

		fakeBuildDB = new(fakes.FakeBuildDB)
		fakeLocker = new(fakes.FakeBuildLocker)

		dbEngine = NewDBEngine(fakeEngine, fakeBuildDB, fakeLocker)
	})

	Describe("CreateBuild", func() {
		var (
			build db.Build
			plan  atc.BuildPlan

			createdBuild Build
			buildErr     error
		)

		BeforeEach(func() {
			build = db.Build{
				ID:   128,
				Name: "some-build",
			}

			plan = atc.BuildPlan{
				Config: &atc.BuildConfig{
					Image: "some-image",

					Params: map[string]string{
						"FOO": "1",
						"BAR": "2",
					},

					Run: atc.BuildRunConfig{
						Path: "some-script",
						Args: []string{"arg1", "arg2"},
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

				fakeEngine.CreateBuildReturns(fakeBuild, nil)
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
				Ω(engine).Should(Equal("fake-engine"))
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
				fakeEngine.CreateBuildReturns(nil, disaster)
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
	})

	Describe("Builds", func() {
		var (
			build Build
			model db.Build
		)

		BeforeEach(func() {
			model = db.Build{
				ID: 128,

				Engine: "fake-engine",
			}

			var err error
			build, err = dbEngine.LookupBuild(model)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Describe("Hijack", func() {
			var (
				hijackSpec garden.ProcessSpec
				hijackIO   garden.ProcessIO

				hijackedProcess garden.Process
				hijackErr       error
			)

			BeforeEach(func() {
				hijackSpec = garden.ProcessSpec{
					Path: "ls",
				}

				hijackIO = garden.ProcessIO{
					Stdin: bytes.NewBufferString("lol"),
				}
			})

			JustBeforeEach(func() {
				hijackedProcess, hijackErr = build.Hijack(hijackSpec, hijackIO)
			})

			Context("when the engine build exists", func() {
				var realBuild *fakes.FakeBuild

				BeforeEach(func() {
					fakeBuildDB.GetBuildReturns(model, nil)

					realBuild = new(fakes.FakeBuild)
					fakeEngine.LookupBuildReturns(realBuild, nil)
				})

				Context("when hijacking the real build succeeds", func() {
					var fakeProcess *gardenfakes.FakeProcess

					BeforeEach(func() {
						fakeProcess = new(gardenfakes.FakeProcess)
						realBuild.HijackReturns(fakeProcess, nil)
					})

					It("succeeds", func() {
						Ω(hijackErr).ShouldNot(HaveOccurred())

						hijackedSpec, hijackedIO := realBuild.HijackArgsForCall(0)
						Ω(hijackedSpec).Should(Equal(hijackSpec))
						Ω(hijackedIO).Should(Equal(hijackIO))
					})

					It("returns the hijacked process", func() {
						Ω(hijackedProcess).Should(Equal(fakeProcess))
					})
				})

				Context("when hijacking the real build fails", func() {
					disaster := errors.New("oh no!")

					BeforeEach(func() {
						realBuild.HijackReturns(nil, disaster)
					})

					It("returns the error", func() {
						Ω(hijackErr).Should(Equal(disaster))
					})
				})
			})

			Context("when the build is not yet active", func() {
				BeforeEach(func() {
					model.Engine = ""
					fakeBuildDB.GetBuildReturns(model, nil)
				})

				It("returns ErrBuildNotActive", func() {
					Ω(hijackErr).Should(Equal(ErrBuildNotActive))
				})
			})

			Context("when looking up the engine build fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeBuildDB.GetBuildReturns(model, nil)
					fakeEngine.LookupBuildReturns(nil, disaster)
				})

				It("returns the error", func() {
					Ω(hijackErr).Should(Equal(disaster))
				})
			})
		})

		Describe("Abort", func() {
			var abortErr error

			JustBeforeEach(func() {
				abortErr = build.Abort()
			})

			Context("when acquiring the lock succeeds", func() {
				var fakeLock *dbfakes.FakeLock

				BeforeEach(func() {
					fakeLock = new(dbfakes.FakeLock)
					fakeLocker.AcquireWriteLockImmediatelyReturns(fakeLock, nil)
				})

				Context("when the build is active", func() {
					BeforeEach(func() {
						model.Engine = "fake-engine"
						fakeBuildDB.GetBuildReturns(model, nil)

						fakeBuildDB.AbortBuildStub = func(int) error {
							Ω(fakeLocker.AcquireWriteLockImmediatelyCallCount()).Should(Equal(1))

							lockedBuild := fakeLocker.AcquireWriteLockImmediatelyArgsForCall(0)
							Ω(lockedBuild).Should(Equal([]db.NamedLock{db.BuildTrackingLock(model.ID)}))

							Ω(fakeLock.ReleaseCallCount()).Should(BeZero())

							return nil
						}
					})

					Context("when the engine build exists", func() {
						var realBuild *fakes.FakeBuild

						BeforeEach(func() {
							fakeBuildDB.GetBuildReturns(model, nil)

							realBuild = new(fakes.FakeBuild)
							fakeEngine.LookupBuildReturns(realBuild, nil)
						})

						Context("when aborting the db build succeeds", func() {
							BeforeEach(func() {
								fakeBuildDB.AbortBuildReturns(nil)
							})

							It("succeeds", func() {
								Ω(abortErr).ShouldNot(HaveOccurred())
							})

							It("releases the lock", func() {
								Ω(fakeLock.ReleaseCallCount()).Should(Equal(1))
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

							It("releases the lock", func() {
								Ω(fakeLock.ReleaseCallCount()).Should(Equal(1))
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
								Ω(fakeLock.ReleaseCallCount()).Should(Equal(1))
							})
						})
					})

					Context("when looking up the engine build fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeBuildDB.GetBuildReturns(model, nil)
							fakeEngine.LookupBuildReturns(nil, disaster)
						})

						It("returns the error", func() {
							Ω(abortErr).Should(Equal(disaster))
						})

						It("releases the lock", func() {
							Ω(fakeLock.ReleaseCallCount()).Should(Equal(1))
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

					It("releases the lock", func() {
						Ω(fakeLock.ReleaseCallCount()).Should(Equal(1))
					})
				})
			})

			Context("when acquiring the lock fails", func() {
				BeforeEach(func() {
					fakeLocker.AcquireWriteLockImmediatelyReturns(nil, errors.New("no lock for you"))
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
						Ω(fakeEngine.LookupBuildCallCount()).Should(BeZero())
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
			var (
				logger lager.Logger

				resumeErr error
			)

			BeforeEach(func() {
				logger = lagertest.NewTestLogger("test")
			})

			JustBeforeEach(func() {
				resumeErr = build.Resume(logger)
			})

			Context("when acquiring the lock succeeds", func() {
				var fakeLock *dbfakes.FakeLock

				BeforeEach(func() {
					fakeLock = new(dbfakes.FakeLock)
					fakeLocker.AcquireWriteLockImmediatelyReturns(fakeLock, nil)
				})

				Context("when the build is active", func() {
					BeforeEach(func() {
						model.Engine = "fake-engine"
						fakeBuildDB.GetBuildReturns(model, nil)
					})

					Context("when the engine build exists", func() {
						var realBuild *fakes.FakeBuild

						BeforeEach(func() {
							fakeBuildDB.GetBuildReturns(model, nil)

							realBuild = new(fakes.FakeBuild)
							fakeEngine.LookupBuildReturns(realBuild, nil)

							realBuild.ResumeStub = func(lager.Logger) error {
								Ω(fakeLocker.AcquireWriteLockImmediatelyCallCount()).Should(Equal(1))

								lockedBuild := fakeLocker.AcquireWriteLockImmediatelyArgsForCall(0)
								Ω(lockedBuild).Should(Equal([]db.NamedLock{db.BuildTrackingLock(model.ID)}))

								Ω(fakeLock.ReleaseCallCount()).Should(BeZero())
								return nil
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

							It("releases the lock", func() {
								Ω(fakeLock.ReleaseCallCount()).Should(Equal(1))
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

									realBuild.ResumeStub = func(lager.Logger) error {
										return <-aborted
									}

									close(abort)
								})

								It("returns whatever Resume returns", func() {
									Ω(resumeErr).Should(Equal(errAborted))
								})

								It("aborts the build", func() {
									Ω(realBuild.AbortCallCount()).Should(Equal(1))
								})

								It("releases the lock", func() {
									Ω(fakeLock.ReleaseCallCount()).Should(Equal(1))
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

							It("returns the error", func() {
								Ω(resumeErr).Should(Equal(disaster))
							})

							It("does not resume the build", func() {
								Ω(realBuild.ResumeCallCount()).Should(BeZero())
							})

							It("releases the lock", func() {
								Ω(fakeLock.ReleaseCallCount()).Should(Equal(1))
							})
						})
					})

					Context("when looking up the engine build fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeBuildDB.GetBuildReturns(model, nil)
							fakeEngine.LookupBuildReturns(nil, disaster)
						})

						It("returns the error", func() {
							Ω(resumeErr).Should(Equal(disaster))
						})

						It("releases the lock", func() {
							Ω(fakeLock.ReleaseCallCount()).Should(Equal(1))
						})
					})
				})

				Context("when the build is not yet active", func() {
					BeforeEach(func() {
						model.Engine = ""
						fakeBuildDB.GetBuildReturns(model, nil)
					})

					It("succeeds", func() {
						Ω(resumeErr).ShouldNot(HaveOccurred())
					})

					It("does not look up the build in the engine", func() {
						Ω(fakeEngine.LookupBuildCallCount()).Should(BeZero())
					})

					It("releases the lock", func() {
						Ω(fakeLock.ReleaseCallCount()).Should(Equal(1))
					})
				})
			})

			Context("when acquiring the lock fails", func() {
				BeforeEach(func() {
					fakeLocker.AcquireWriteLockImmediatelyReturns(nil, errors.New("no lock for you"))
				})

				It("succeeds", func() {
					Ω(resumeErr).ShouldNot(HaveOccurred())
				})

				It("does not look up the build", func() {
					Ω(fakeBuildDB.GetBuildCallCount()).Should(BeZero())
					Ω(fakeEngine.LookupBuildCallCount()).Should(BeZero())
				})
			})
		})
	})
})
