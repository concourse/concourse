package buildstarter_test

import (
	"errors"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/engine/enginefakes"
	"github.com/concourse/atc/scheduler/buildstarter"
	"github.com/concourse/atc/scheduler/buildstarter/buildstarterfakes"
	"github.com/concourse/atc/scheduler/buildstarter/maxinflight/maxinflightfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("I'm a BuildStarter", func() {
	var (
		fakeDB      *buildstarterfakes.FakeBuildStarterDB
		fakeUpdater *maxinflightfakes.FakeUpdater
		fakeFactory *buildstarterfakes.FakeBuildFactory
		fakeEngine  *enginefakes.FakeEngine

		buildStarter buildstarter.BuildStarter

		disaster error
	)

	BeforeEach(func() {
		fakeDB = new(buildstarterfakes.FakeBuildStarterDB)
		fakeUpdater = new(maxinflightfakes.FakeUpdater)
		fakeFactory = new(buildstarterfakes.FakeBuildFactory)
		fakeEngine = new(enginefakes.FakeEngine)

		buildStarter = buildstarter.NewBuildStarter(fakeDB, fakeUpdater, fakeFactory, fakeEngine)

		disaster = errors.New("bad thing")
	})

	Describe("TryStartAllPendingBuilds", func() {
		var tryStartErr error

		JustBeforeEach(func() {
			tryStartErr = buildStarter.TryStartAllPendingBuilds(
				lagertest.NewTestLogger("test"),
				atc.JobConfig{Name: "some-job"},
				atc.ResourceConfigs{{Name: "some-resource"}},
				atc.ResourceTypes{{Name: "some-resource-type"}})
		})

		Context("when the stars align", func() {
			BeforeEach(func() {
				fakeUpdater.UpdateMaxInFlightReachedReturns(false, nil)
				fakeDB.GetNextBuildInputsReturns([]db.BuildInput{{Name: "some-input"}}, true, nil)
				fakeDB.IsPausedReturns(false, nil)
				fakeDB.GetJobReturns(db.SavedJob{Paused: false}, nil)
			})

			Context("when getting the next pending build fails", func() {
				BeforeEach(func() {
					fakeDB.GetNextPendingBuildReturns(nil, false, disaster)
				})

				It("returns the error", func() {
					Expect(tryStartErr).To(Equal(disaster))
				})

				It("got the pending build for the right job", func() {
					Expect(fakeDB.GetNextPendingBuildCallCount()).To(Equal(1))
					Expect(fakeDB.GetNextPendingBuildArgsForCall(0)).To(Equal("some-job"))
				})
			})

			Context("when there is no pending build", func() {
				BeforeEach(func() {
					fakeDB.GetNextPendingBuildReturns(nil, false, nil)
				})

				It("doesn't return an error", func() {
					Expect(tryStartErr).NotTo(HaveOccurred())
				})
			})

			Context("when there is a pending build", func() {
				var pendingBuild *dbfakes.FakeBuild
				var pendingBuildCount int

				BeforeEach(func() {
					pendingBuild = new(dbfakes.FakeBuild)
					pendingBuild.IDReturns(99)
					pendingBuildCount = 1
					fakeDB.GetNextPendingBuildStub = func(string) (db.Build, bool, error) {
						if fakeDB.GetNextBuildInputsCallCount() < pendingBuildCount {
							return pendingBuild, true, nil
						}
						return nil, false, nil
					}
				})

				Context("when marking the build as scheduled fails", func() {
					BeforeEach(func() {
						fakeDB.UpdateBuildToScheduledReturns(false, disaster)
					})

					It("returns the error", func() {
						Expect(tryStartErr).To(Equal(disaster))
					})

					It("marked the right build as scheduled", func() {
						Expect(fakeDB.UpdateBuildToScheduledCallCount()).To(Equal(1))
						Expect(fakeDB.UpdateBuildToScheduledArgsForCall(0)).To(Equal(99))
					})
				})

				Context("when someone else already scheduled the build", func() {
					BeforeEach(func() {
						fakeDB.UpdateBuildToScheduledReturns(false, nil)
					})

					It("doesn't return an error", func() {
						Expect(tryStartErr).NotTo(HaveOccurred())
					})

					It("doesn't try to use inputs for build", func() {
						Expect(fakeDB.UseInputsForBuildCallCount()).To(BeZero())
					})
				})

				Context("when marking the build as scheduled succeeds", func() {
					BeforeEach(func() {
						fakeDB.UpdateBuildToScheduledReturns(true, nil)
					})

					Context("when using inputs for build fails", func() {
						BeforeEach(func() {
							fakeDB.UseInputsForBuildReturns(disaster)
						})

						It("returns the error", func() {
							Expect(tryStartErr).To(Equal(disaster))
						})

						It("used the right inputs for the right build", func() {
							Expect(fakeDB.UseInputsForBuildCallCount()).To(Equal(1))
							actualBuildID, actualInputs := fakeDB.UseInputsForBuildArgsForCall(0)
							Expect(actualBuildID).To(Equal(99))
							Expect(actualInputs).To(Equal([]db.BuildInput{{Name: "some-input"}}))
						})
					})

					Context("when using inputs for build succeeds", func() {
						BeforeEach(func() {
							fakeDB.UseInputsForBuildReturns(nil)
						})

						Context("when creating the build plan fails", func() {
							BeforeEach(func() {
								fakeFactory.CreateReturns(atc.Plan{}, disaster)
							})

							It("created the build plan with the right config", func() {
								Expect(fakeFactory.CreateCallCount()).To(Equal(1))
								actualJobConfig, actualResourceConfigs, actualResourceTypes, actualBuildInputs := fakeFactory.CreateArgsForCall(0)
								Expect(actualJobConfig).To(Equal(atc.JobConfig{Name: "some-job"}))
								Expect(actualResourceConfigs).To(Equal(atc.ResourceConfigs{{Name: "some-resource"}}))
								Expect(actualResourceTypes).To(Equal(atc.ResourceTypes{{Name: "some-resource-type"}}))
								Expect(actualBuildInputs).To(Equal([]db.BuildInput{{Name: "some-input"}}))
							})

							Context("when marking the build as errored fails", func() {
								BeforeEach(func() {
									pendingBuild.FinishReturns(disaster)
								})

								It("doesn't return an error", func() {
									Expect(tryStartErr).NotTo(HaveOccurred())
								})

								It("marked the right build as errored", func() {
									Expect(pendingBuild.FinishCallCount()).To(Equal(1))
									actualStatus := pendingBuild.FinishArgsForCall(0)
									Expect(actualStatus).To(Equal(db.StatusErrored))
								})
							})

							Context("when marking the build as errored succeeds", func() {
								BeforeEach(func() {
									pendingBuild.FinishReturns(nil)
								})

								It("doesn't return an error", func() {
									Expect(tryStartErr).NotTo(HaveOccurred())
								})
							})
						})

						Context("when creating the build plan succeeds", func() {
							BeforeEach(func() {
								fakeFactory.CreateReturns(atc.Plan{Task: &atc.TaskPlan{ConfigPath: "some-task.yml"}}, nil)
							})

							Context("when creating the engine build fails", func() {
								BeforeEach(func() {
									fakeEngine.CreateBuildReturns(nil, disaster)
								})

								It("doesn't return an error", func() {
									Expect(tryStartErr).NotTo(HaveOccurred())
								})

								It("created the engine build with the right build and plan", func() {
									Expect(fakeEngine.CreateBuildCallCount()).To(Equal(1))
									_, actualBuild, actualPlan := fakeEngine.CreateBuildArgsForCall(0)
									Expect(actualBuild).To(Equal(pendingBuild))
									Expect(actualPlan).To(Equal(atc.Plan{Task: &atc.TaskPlan{ConfigPath: "some-task.yml"}}))
								})
							})

							Context("when creating the engine build succeeds", func() {
								var engineBuild *enginefakes.FakeBuild

								BeforeEach(func() {
									engineBuild = new(enginefakes.FakeBuild)
									fakeEngine.CreateBuildReturns(engineBuild, nil)
								})

								It("doesn't return an error", func() {
									Expect(tryStartErr).NotTo(HaveOccurred())
								})

								It("starts the engine build (asynchronously)", func() {
									Eventually(engineBuild.ResumeCallCount).Should(Equal(1))
								})

								Context("when there are 7 pending builds", func() {
									BeforeEach(func() {
										pendingBuildCount = 7
									})

									It("starts 7 engine builds (asynchronously)", func() {
										Eventually(engineBuild.ResumeCallCount).Should(Equal(7))
									})
								})
							})
						})
					})
				})

				itReturnsTheError := func() {
					It("returns the error", func() {
						Expect(tryStartErr).To(Equal(disaster))
					})
				}

				itDoesntReturnAnErrorOrMarkTheBuildAsScheduled := func() {
					It("doesn't return an error", func() {
						Expect(tryStartErr).NotTo(HaveOccurred())
					})

					It("doesn't try to mark the build as scheduled", func() {
						Expect(fakeDB.UpdateBuildToScheduledCallCount()).To(BeZero())
					})
				}

				itUpdatedMaxInFlightForTheRightJob := func() {
					It("updated max in flight for the right job", func() {
						Expect(fakeUpdater.UpdateMaxInFlightReachedCallCount()).To(Equal(1))
						_, actualJobConfig, actualBuildID := fakeUpdater.UpdateMaxInFlightReachedArgsForCall(0)
						Expect(actualJobConfig).To(Equal(atc.JobConfig{Name: "some-job"}))
						Expect(actualBuildID).To(Equal(99))
					})
				}

				Context("when updating max in flight reached fails", func() {
					BeforeEach(func() {
						fakeUpdater.UpdateMaxInFlightReachedReturns(false, disaster)
					})

					itReturnsTheError()
					itUpdatedMaxInFlightForTheRightJob()
				})

				Context("when max in flight is reached", func() {
					BeforeEach(func() {
						fakeUpdater.UpdateMaxInFlightReachedReturns(true, nil)
					})

					itDoesntReturnAnErrorOrMarkTheBuildAsScheduled()
				})

				Context("when getting the next build inputs fails", func() {
					BeforeEach(func() {
						fakeDB.GetNextBuildInputsReturns(nil, false, disaster)
					})

					itReturnsTheError()
					itUpdatedMaxInFlightForTheRightJob()
				})

				Context("when there are no next build inputs", func() {
					BeforeEach(func() {
						fakeDB.GetNextBuildInputsReturns(nil, false, nil)
					})

					itDoesntReturnAnErrorOrMarkTheBuildAsScheduled()
					itUpdatedMaxInFlightForTheRightJob()
				})

				Context("when checking if the pipeline is paused fails", func() {
					BeforeEach(func() {
						fakeDB.IsPausedReturns(false, disaster)
					})

					itReturnsTheError()
					itUpdatedMaxInFlightForTheRightJob()
				})

				Context("when the pipeline is paused", func() {
					BeforeEach(func() {
						fakeDB.IsPausedReturns(true, nil)
					})

					itDoesntReturnAnErrorOrMarkTheBuildAsScheduled()
					itUpdatedMaxInFlightForTheRightJob()
				})

				Context("when getting the job fails", func() {
					BeforeEach(func() {
						fakeDB.GetJobReturns(db.SavedJob{}, disaster)
					})

					itReturnsTheError()
					itUpdatedMaxInFlightForTheRightJob()
				})

				Context("when the job is paused", func() {
					BeforeEach(func() {
						fakeDB.GetJobReturns(db.SavedJob{Paused: true}, nil)
					})

					itDoesntReturnAnErrorOrMarkTheBuildAsScheduled()
					itUpdatedMaxInFlightForTheRightJob()
				})
			})
		})
	})
})
