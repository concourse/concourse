package buildstarter_test

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/engine"
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
				atc.JobConfigs{
					{Name: "some-job-without-pending-builds"},
					{Name: "some-job-1"},
					{Name: "some-job-2"},
				},
				atc.ResourceConfigs{{Name: "some-resource"}},
				atc.ResourceTypes{{Name: "some-resource-type"}})
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

		itUpdatedMaxInFlightForAllJobs := func() {
			It("updated max in flight for the right jobs", func() {
				Expect(fakeUpdater.UpdateMaxInFlightReachedCallCount()).To(Equal(3))
				_, actualJobConfig, actualBuildID := fakeUpdater.UpdateMaxInFlightReachedArgsForCall(0)
				Expect(actualJobConfig).To(Equal(atc.JobConfig{Name: "some-job-1"}))
				Expect(actualBuildID).To(Equal(99))

				_, actualJobConfig, actualBuildID = fakeUpdater.UpdateMaxInFlightReachedArgsForCall(1)
				Expect(actualJobConfig).To(Equal(atc.JobConfig{Name: "some-job-1"}))
				Expect(actualBuildID).To(Equal(999))

				_, actualJobConfig, actualBuildID = fakeUpdater.UpdateMaxInFlightReachedArgsForCall(2)
				Expect(actualJobConfig).To(Equal(atc.JobConfig{Name: "some-job-2"}))
				Expect(actualBuildID).To(Equal(555))
			})
		}

		itUpdatedMaxInFlightForFirstBuildOfEachJob := func() {
			It("updated max in flight for the right jobs", func() {
				Expect(fakeUpdater.UpdateMaxInFlightReachedCallCount()).To(Equal(2))
				_, actualJobConfig, actualBuildID := fakeUpdater.UpdateMaxInFlightReachedArgsForCall(0)
				Expect(actualJobConfig).To(Equal(atc.JobConfig{Name: "some-job-1"}))
				Expect(actualBuildID).To(Equal(99))

				_, actualJobConfig, actualBuildID = fakeUpdater.UpdateMaxInFlightReachedArgsForCall(1)
				Expect(actualJobConfig).To(Equal(atc.JobConfig{Name: "some-job-2"}))
				Expect(actualBuildID).To(Equal(555))
			})
		}

		itUpdatedMaxInFlightForTheFirstJob := func() {
			It("updated max in flight for the first jobs", func() {
				Expect(fakeUpdater.UpdateMaxInFlightReachedCallCount()).To(Equal(1))
				_, actualJobConfig, actualBuildID := fakeUpdater.UpdateMaxInFlightReachedArgsForCall(0)
				Expect(actualJobConfig).To(Equal(atc.JobConfig{Name: "some-job-1"}))
				Expect(actualBuildID).To(Equal(99))
			})
		}

		Context("when the stars align", func() {
			BeforeEach(func() {
				fakeUpdater.UpdateMaxInFlightReachedReturns(false, nil)
				fakeDB.GetNextBuildInputsReturns([]db.BuildInput{{Name: "some-input"}}, true, nil)
				fakeDB.IsPausedReturns(false, nil)
				fakeDB.GetJobReturns(db.SavedJob{Paused: false}, true, nil)
			})

			Context("when getting the next pending build fails", func() {
				BeforeEach(func() {
					fakeDB.GetAllPendingBuildsReturns(nil, disaster)
				})

				It("returns the error", func() {
					Expect(tryStartErr).To(Equal(disaster))
				})

				It("got the pending build for the right job", func() {
					Expect(fakeDB.GetAllPendingBuildsCallCount()).To(Equal(1))
				})
			})

			Context("when there is no pending build", func() {
				BeforeEach(func() {
					fakeDB.GetAllPendingBuildsReturns(nil, nil)
				})

				It("doesn't return an error", func() {
					Expect(tryStartErr).NotTo(HaveOccurred())
				})
			})

			Context("when there is a pending build", func() {
				var pendingBuild1 *dbfakes.FakeBuild
				var pendingBuild2 *dbfakes.FakeBuild
				var pendingBuild3 *dbfakes.FakeBuild

				BeforeEach(func() {
					pendingBuild1 = new(dbfakes.FakeBuild)
					pendingBuild1.IDReturns(99)
					pendingBuild2 = new(dbfakes.FakeBuild)
					pendingBuild2.IDReturns(999)
					pendingBuild3 = new(dbfakes.FakeBuild)
					pendingBuild3.IDReturns(555)

					fakeDB.GetAllPendingBuildsStub = func() (map[string][]db.Build, error) {
						return map[string][]db.Build{
							"some-job-1": []db.Build{pendingBuild1, pendingBuild2},
							"some-job-2": []db.Build{pendingBuild3},
						}, nil
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

							It("stops creating builds for job", func() {
								Expect(fakeFactory.CreateCallCount()).To(Equal(2))
								actualJobConfig, actualResourceConfigs, actualResourceTypes, actualBuildInputs := fakeFactory.CreateArgsForCall(0)
								Expect(actualJobConfig).To(Equal(atc.JobConfig{Name: "some-job-1"}))
								Expect(actualResourceConfigs).To(Equal(atc.ResourceConfigs{{Name: "some-resource"}}))
								Expect(actualResourceTypes).To(Equal(atc.ResourceTypes{{Name: "some-resource-type"}}))
								Expect(actualBuildInputs).To(Equal([]db.BuildInput{{Name: "some-input"}}))

								actualJobConfig, actualResourceConfigs, actualResourceTypes, actualBuildInputs = fakeFactory.CreateArgsForCall(1)
								Expect(actualJobConfig).To(Equal(atc.JobConfig{Name: "some-job-2"}))
								Expect(actualResourceConfigs).To(Equal(atc.ResourceConfigs{{Name: "some-resource"}}))
								Expect(actualResourceTypes).To(Equal(atc.ResourceTypes{{Name: "some-resource-type"}}))
								Expect(actualBuildInputs).To(Equal([]db.BuildInput{{Name: "some-input"}}))
							})

							Context("when marking the build as errored fails", func() {
								BeforeEach(func() {
									pendingBuild1.FinishReturns(disaster)
								})

								It("doesn't return an error", func() {
									Expect(tryStartErr).NotTo(HaveOccurred())
								})

								It("marked the right build as errored", func() {
									Expect(pendingBuild1.FinishCallCount()).To(Equal(1))
									actualStatus := pendingBuild1.FinishArgsForCall(0)
									Expect(actualStatus).To(Equal(db.StatusErrored))
								})
							})

							Context("when marking the build as errored succeeds", func() {
								BeforeEach(func() {
									pendingBuild1.FinishReturns(nil)
								})

								It("doesn't return an error", func() {
									Expect(tryStartErr).NotTo(HaveOccurred())
								})
							})
						})

						Context("when creating the build plan succeeds", func() {
							BeforeEach(func() {
								fakeFactory.CreateStub = func(jobConfig atc.JobConfig, resoruceConfigs atc.ResourceConfigs, resourceTypes atc.ResourceTypes, buildInputs []db.BuildInput) (atc.Plan, error) {
									if jobConfig.Name == "some-job-1" {
										return atc.Plan{Task: &atc.TaskPlan{ConfigPath: "some-task-1.yml"}}, nil
									}

									if jobConfig.Name == "some-job-2" {
										return atc.Plan{Task: &atc.TaskPlan{ConfigPath: "some-task-2.yml"}}, nil
									}

									panic("unknown-job")
								}

								fakeEngine.CreateBuildReturns(new(enginefakes.FakeBuild), nil)
							})

							It("creates build plans for all builds", func() {
								Expect(fakeFactory.CreateCallCount()).To(Equal(3))
								actualJobConfig, actualResourceConfigs, actualResourceTypes, actualBuildInputs := fakeFactory.CreateArgsForCall(0)
								Expect(actualJobConfig).To(Equal(atc.JobConfig{Name: "some-job-1"}))
								Expect(actualResourceConfigs).To(Equal(atc.ResourceConfigs{{Name: "some-resource"}}))
								Expect(actualResourceTypes).To(Equal(atc.ResourceTypes{{Name: "some-resource-type"}}))
								Expect(actualBuildInputs).To(Equal([]db.BuildInput{{Name: "some-input"}}))

								actualJobConfig, actualResourceConfigs, actualResourceTypes, actualBuildInputs = fakeFactory.CreateArgsForCall(1)
								Expect(actualJobConfig).To(Equal(atc.JobConfig{Name: "some-job-1"}))
								Expect(actualResourceConfigs).To(Equal(atc.ResourceConfigs{{Name: "some-resource"}}))
								Expect(actualResourceTypes).To(Equal(atc.ResourceTypes{{Name: "some-resource-type"}}))
								Expect(actualBuildInputs).To(Equal([]db.BuildInput{{Name: "some-input"}}))

								actualJobConfig, actualResourceConfigs, actualResourceTypes, actualBuildInputs = fakeFactory.CreateArgsForCall(2)
								Expect(actualJobConfig).To(Equal(atc.JobConfig{Name: "some-job-2"}))
								Expect(actualResourceConfigs).To(Equal(atc.ResourceConfigs{{Name: "some-resource"}}))
								Expect(actualResourceTypes).To(Equal(atc.ResourceTypes{{Name: "some-resource-type"}}))
								Expect(actualBuildInputs).To(Equal([]db.BuildInput{{Name: "some-input"}}))
							})

							Context("when creating the engine build fails", func() {
								BeforeEach(func() {
									fakeEngine.CreateBuildReturns(nil, disaster)
								})

								It("doesn't return an error", func() {
									Expect(tryStartErr).NotTo(HaveOccurred())
								})
							})

							Context("when creating the engine build succeeds", func() {
								var engineBuild1 *enginefakes.FakeBuild
								var engineBuild2 *enginefakes.FakeBuild
								var engineBuild3 *enginefakes.FakeBuild

								BeforeEach(func() {
									engineBuild1 = new(enginefakes.FakeBuild)
									engineBuild2 = new(enginefakes.FakeBuild)
									engineBuild3 = new(enginefakes.FakeBuild)
									createBuildCallCount := 0
									fakeEngine.CreateBuildStub = func(lager.Logger, db.Build, atc.Plan) (engine.Build, error) {
										createBuildCallCount++
										switch createBuildCallCount {
										case 1:
											return engineBuild1, nil
										case 2:
											return engineBuild2, nil
										case 3:
											return engineBuild3, nil
										default:
											panic("unexpected-call-count-for-create-build")
										}
									}
								})

								It("doesn't return an error", func() {
									Expect(tryStartErr).NotTo(HaveOccurred())
								})

								itUpdatedMaxInFlightForAllJobs()

								It("created the engine build with the right build and plan", func() {
									Expect(fakeEngine.CreateBuildCallCount()).To(Equal(3))
									_, actualBuild, actualPlan := fakeEngine.CreateBuildArgsForCall(0)
									Expect(actualBuild).To(Equal(pendingBuild1))
									Expect(actualPlan).To(Equal(atc.Plan{Task: &atc.TaskPlan{ConfigPath: "some-task-1.yml"}}))

									_, actualBuild, actualPlan = fakeEngine.CreateBuildArgsForCall(1)
									Expect(actualBuild).To(Equal(pendingBuild2))
									Expect(actualPlan).To(Equal(atc.Plan{Task: &atc.TaskPlan{ConfigPath: "some-task-1.yml"}}))

									_, actualBuild, actualPlan = fakeEngine.CreateBuildArgsForCall(2)
									Expect(actualBuild).To(Equal(pendingBuild3))
									Expect(actualPlan).To(Equal(atc.Plan{Task: &atc.TaskPlan{ConfigPath: "some-task-2.yml"}}))
								})

								It("starts the engine build (asynchronously)", func() {
									Eventually(engineBuild1.ResumeCallCount).Should(Equal(1))
									Eventually(engineBuild2.ResumeCallCount).Should(Equal(1))
									Eventually(engineBuild3.ResumeCallCount).Should(Equal(1))
								})
							})
						})
					})
				})

				Context("when updating max in flight reached fails", func() {
					BeforeEach(func() {
						fakeUpdater.UpdateMaxInFlightReachedReturns(false, disaster)
					})

					itReturnsTheError()
					itUpdatedMaxInFlightForTheFirstJob()
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
					itUpdatedMaxInFlightForTheFirstJob()
				})

				Context("when there are no next build inputs", func() {
					BeforeEach(func() {
						fakeDB.GetNextBuildInputsReturns(nil, false, nil)
					})

					itDoesntReturnAnErrorOrMarkTheBuildAsScheduled()
					itUpdatedMaxInFlightForFirstBuildOfEachJob()
				})

				Context("when checking if the pipeline is paused fails", func() {
					BeforeEach(func() {
						fakeDB.IsPausedReturns(false, disaster)
					})

					itReturnsTheError()
					itUpdatedMaxInFlightForTheFirstJob()
				})

				Context("when the pipeline is paused", func() {
					BeforeEach(func() {
						fakeDB.IsPausedReturns(true, nil)
					})

					itDoesntReturnAnErrorOrMarkTheBuildAsScheduled()
					itUpdatedMaxInFlightForFirstBuildOfEachJob()
				})

				Context("when getting the job fails", func() {
					BeforeEach(func() {
						fakeDB.GetJobReturns(db.SavedJob{}, false, disaster)
					})

					itReturnsTheError()
					itUpdatedMaxInFlightForTheFirstJob()
				})

				Context("when the job is paused", func() {
					BeforeEach(func() {
						fakeDB.GetJobReturns(db.SavedJob{Paused: true}, true, nil)
					})

					itDoesntReturnAnErrorOrMarkTheBuildAsScheduled()
					itUpdatedMaxInFlightForFirstBuildOfEachJob()
				})
			})
		})
	})
})
