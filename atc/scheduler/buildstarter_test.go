package scheduler_test

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/enginefakes"
	"github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/inputmapper/inputmapperfakes"
	"github.com/concourse/atc/scheduler/maxinflight/maxinflightfakes"
	"github.com/concourse/atc/scheduler/schedulerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildStarter", func() {
	var (
		fakePipeline    *dbfakes.FakePipeline
		fakeUpdater     *maxinflightfakes.FakeUpdater
		fakeFactory     *schedulerfakes.FakeBuildFactory
		fakeEngine      *enginefakes.FakeEngine
		pendingBuilds   []db.Build
		fakeScanner     *schedulerfakes.FakeScanner
		fakeInputMapper *inputmapperfakes.FakeInputMapper

		buildStarter scheduler.BuildStarter

		disaster error
	)

	BeforeEach(func() {
		fakePipeline = new(dbfakes.FakePipeline)
		fakeUpdater = new(maxinflightfakes.FakeUpdater)
		fakeFactory = new(schedulerfakes.FakeBuildFactory)
		fakeEngine = new(enginefakes.FakeEngine)
		fakeScanner = new(schedulerfakes.FakeScanner)
		fakeInputMapper = new(inputmapperfakes.FakeInputMapper)

		buildStarter = scheduler.NewBuildStarter(fakePipeline, fakeUpdater, fakeFactory, fakeScanner, fakeInputMapper, fakeEngine)

		disaster = errors.New("bad thing")
	})

	Describe("TryStartPendingBuildsForJob", func() {
		var tryStartErr error
		var createdBuild *dbfakes.FakeBuild
		var job *dbfakes.FakeJob
		var resource *dbfakes.FakeResource
		var versionedResourceTypes atc.VersionedResourceTypes

		BeforeEach(func() {
			versionedResourceTypes = atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{Name: "some-resource-type"},
					Version:      atc.Version{"some": "version"},
				},
			}

			createdBuild = new(dbfakes.FakeBuild)
			createdBuild.IDReturns(66)
			createdBuild.IsManuallyTriggeredReturns(true)

			pendingBuilds = []db.Build{createdBuild}

			resource = new(dbfakes.FakeResource)
			resource.NameReturns("some-resource")
		})

		Context("when manually triggered", func() {
			BeforeEach(func() {
				job = new(dbfakes.FakeJob)
				job.NameReturns("some-job")
				job.ConfigReturns(atc.JobConfig{Plan: atc.PlanSequence{{Get: "input-1"}, {Get: "input-2"}}})
			})

			JustBeforeEach(func() {
				tryStartErr = buildStarter.TryStartPendingBuildsForJob(
					lagertest.NewTestLogger("test"),
					job,
					db.Resources{resource},
					versionedResourceTypes,
					pendingBuilds,
				)
			})

			It("updates max in flight for the job", func() {
				Expect(fakeUpdater.UpdateMaxInFlightReachedCallCount()).To(Equal(1))
				_, actualJob, actualBuildID := fakeUpdater.UpdateMaxInFlightReachedArgsForCall(0)
				Expect(actualJob.Name()).To(Equal(job.Name()))
				Expect(actualBuildID).To(Equal(66))
			})

			Context("when max in flight is reached", func() {
				BeforeEach(func() {
					fakeUpdater.UpdateMaxInFlightReachedReturns(true, nil)
				})

				It("does not run resource check", func() {
					Expect(fakeScanner.ScanCallCount()).To(Equal(0))
				})
			})

			Context("when max in flight is not reached", func() {
				BeforeEach(func() {
					fakeUpdater.UpdateMaxInFlightReachedReturns(false, nil)
				})

				It("runs resource check for every job resource", func() {
					Expect(fakeScanner.ScanCallCount()).To(Equal(2))
				})

				Context("when resource checking fails", func() {
					BeforeEach(func() {
						fakeScanner.ScanReturns(disaster)
					})

					It("doesn't reload the resource types list", func() {
						Expect(fakePipeline.ResourceTypesCallCount()).To(Equal(0))
					})

					It("returns an error", func() {
						Expect(tryStartErr).To(Equal(disaster))
					})
				})

				Context("when resource checking succeeds", func() {
					BeforeEach(func() {
						fakeScanner.ScanStub = func(lager.Logger, string) error {
							defer GinkgoRecover()
							Expect(fakePipeline.LoadVersionsDBCallCount()).To(BeZero())
							return nil
						}
						fakeDBResourceType := new(dbfakes.FakeResourceType)
						fakeDBResourceType.NameReturns("fake-resource-type")
						fakeDBResourceType.TypeReturns("fake")
						fakeDBResourceType.SourceReturns(atc.Source{"im": "fake"})
						fakeDBResourceType.PrivilegedReturns(true)
						fakeDBResourceType.VersionReturns(atc.Version{"version": "1.2.3"})

						fakePipeline.ResourceTypesReturns(db.ResourceTypes{fakeDBResourceType}, nil)

					})

					Context("when reloading the resource types list fails", func() {
						BeforeEach(func() {
							fakePipeline.ResourceTypesReturns(db.ResourceTypes{}, errors.New("failed to reload types"))
						})
						It("returns the error", func() {
							Expect(tryStartErr).To(MatchError("failed to reload types"))
						})
					})

					It("reloads the resource types list", func() {
						Expect(fakePipeline.ResourceTypesCallCount()).To(Equal(1))
					})

					Context("when loading the versions DB fails", func() {
						BeforeEach(func() {
							fakePipeline.LoadVersionsDBReturns(nil, disaster)
						})

						It("returns an error", func() {
							Expect(tryStartErr).To(Equal(disaster))
						})

						It("checked for the right resources", func() {
							Expect(fakeScanner.ScanCallCount()).To(Equal(2))
							_, resource1 := fakeScanner.ScanArgsForCall(0)
							_, resource2 := fakeScanner.ScanArgsForCall(1)
							Expect([]string{resource1, resource2}).To(ConsistOf("input-1", "input-2"))
						})

						It("loaded the versions DB after checking all the resources", func() {
							Expect(fakePipeline.LoadVersionsDBCallCount()).To(Equal(1))
						})
					})

					Context("when loading the versions DB succeeds", func() {
						var versionsDB *algorithm.VersionsDB

						BeforeEach(func() {
							fakePipeline.LoadVersionsDBReturns(&algorithm.VersionsDB{
								ResourceVersions: []algorithm.ResourceVersion{
									{
										VersionID:  73,
										ResourceID: 127,
										CheckOrder: 123,
									},
								},
								BuildOutputs: []algorithm.BuildOutput{
									{
										ResourceVersion: algorithm.ResourceVersion{
											VersionID:  73,
											ResourceID: 127,
											CheckOrder: 123,
										},
										BuildID: 66,
										JobID:   13,
									},
								},
								BuildInputs: []algorithm.BuildInput{
									{
										ResourceVersion: algorithm.ResourceVersion{
											VersionID:  66,
											ResourceID: 77,
											CheckOrder: 88,
										},
										BuildID:   66,
										JobID:     13,
										InputName: "some-input-name",
									},
								},
								JobIDs: map[string]int{
									"bad-luck-job": 13,
								},
								ResourceIDs: map[string]int{
									"resource-127": 127,
								},
							}, nil)

							versionsDB = &algorithm.VersionsDB{JobIDs: map[string]int{"j1": 1}}
							fakePipeline.LoadVersionsDBReturns(versionsDB, nil)
						})

						Context("when saving the next input mapping fails", func() {
							BeforeEach(func() {
								fakeInputMapper.SaveNextInputMappingReturns(nil, disaster)
							})

							It("saved the next input mapping for the right job and versions", func() {
								Expect(fakeInputMapper.SaveNextInputMappingCallCount()).To(Equal(1))
								_, actualVersionsDB, actualJob, _ := fakeInputMapper.SaveNextInputMappingArgsForCall(0)
								Expect(actualVersionsDB).To(Equal(versionsDB))
								Expect(actualJob.Name()).To(Equal(job.Name()))
							})
						})

						Context("when saving the next input mapping succeeds", func() {
							BeforeEach(func() {
								fakeInputMapper.SaveNextInputMappingStub = func(lager.Logger, *algorithm.VersionsDB, db.Job, db.Resources) (algorithm.InputMapping, error) {
									defer GinkgoRecover()
									return nil, nil
								}
							})

							It("saved the next input mapping and returns the build", func() {
								Expect(tryStartErr).NotTo(HaveOccurred())
							})

							Context("when creaing a build plan", func() {
								BeforeEach(func() {
									job.GetNextBuildInputsReturns([]db.BuildInput{}, true, nil)
									fakePipeline.CheckPausedReturns(false, nil)
									createdBuild.ScheduleReturns(true, nil)
									createdBuild.UseInputsReturns(nil)
									fakeEngine.CreateBuildReturns(new(enginefakes.FakeBuild), nil)
								})

								It("uses the updated list of resource types", func() {
									Expect(fakeFactory.CreateCallCount()).To(Equal(1))
									_, _, types, _ := fakeFactory.CreateArgsForCall(0)
									Expect(types).To(ConsistOf(atc.VersionedResourceTypes{atc.VersionedResourceType{
										ResourceType: atc.ResourceType{
											Name:       "fake-resource-type",
											Type:       "fake",
											Source:     atc.Source{"im": "fake"},
											Privileged: true,
										},
										Version: atc.Version{"version": "1.2.3"},
									}}))
								})
							})

						})
					})
				})
			})
		})

		Context("when not manually triggered", func() {
			BeforeEach(func() {
				job = new(dbfakes.FakeJob)
				job.NameReturns("some-job")
				job.ConfigReturns(atc.JobConfig{Name: "some-job"})
				createdBuild.IsManuallyTriggeredReturns(false)
			})

			JustBeforeEach(func() {
				tryStartErr = buildStarter.TryStartPendingBuildsForJob(
					lagertest.NewTestLogger("test"),
					job,
					db.Resources{resource},
					atc.VersionedResourceTypes{
						{
							ResourceType: atc.ResourceType{Name: "some-resource-type"},
							Version:      atc.Version{"some": "version"},
						},
					},
					pendingBuilds,
				)
			})

			itReturnsTheError := func() {
				It("returns the error", func() {
					Expect(tryStartErr).To(Equal(disaster))
				})
			}

			It("doesn't reload the resource types list", func() {
				Expect(fakePipeline.ResourceTypesCallCount()).To(Equal(0))
			})

			itDoesntReturnAnErrorOrMarkTheBuildAsScheduled := func() {
				It("doesn't return an error", func() {
					Expect(tryStartErr).NotTo(HaveOccurred())
				})

				It("doesn't try to mark the build as scheduled", func() {
					Expect(createdBuild.ScheduleCallCount()).To(BeZero())
				})
			}

			itUpdatedMaxInFlightForAllBuilds := func() {
				It("updated max in flight for the right jobs", func() {
					Expect(fakeUpdater.UpdateMaxInFlightReachedCallCount()).To(Equal(3))
					_, actualJob, actualBuildID := fakeUpdater.UpdateMaxInFlightReachedArgsForCall(0)
					Expect(actualJob).To(Equal(job))
					Expect(actualBuildID).To(Equal(99))

					_, actualJob, actualBuildID = fakeUpdater.UpdateMaxInFlightReachedArgsForCall(1)
					Expect(actualJob.Name()).To(Equal(job.Name()))
					Expect(actualBuildID).To(Equal(999))
				})
			}

			itUpdatedMaxInFlightForTheFirstBuild := func() {
				It("updated max in flight for the first jobs", func() {
					Expect(fakeUpdater.UpdateMaxInFlightReachedCallCount()).To(Equal(1))
					_, actualJob, actualBuildID := fakeUpdater.UpdateMaxInFlightReachedArgsForCall(0)
					Expect(actualJob.Name()).To(Equal(job.Name()))
					Expect(actualBuildID).To(Equal(99))
				})
			}

			Context("when the stars align", func() {
				BeforeEach(func() {
					job.PausedReturns(false)
					fakeUpdater.UpdateMaxInFlightReachedReturns(false, nil)
					job.GetNextBuildInputsReturns([]db.BuildInput{{Name: "some-input"}}, true, nil)
					fakePipeline.PausedReturns(false)
				})

				Context("when there are several pending builds", func() {
					var pendingBuild1 *dbfakes.FakeBuild
					var pendingBuild2 *dbfakes.FakeBuild
					var pendingBuild3 *dbfakes.FakeBuild

					BeforeEach(func() {
						pendingBuild1 = new(dbfakes.FakeBuild)
						pendingBuild1.IDReturns(99)
						pendingBuild1.ScheduleReturns(true, nil)
						pendingBuild2 = new(dbfakes.FakeBuild)
						pendingBuild2.IDReturns(999)
						pendingBuild2.ScheduleReturns(true, nil)
						pendingBuild3 = new(dbfakes.FakeBuild)
						pendingBuild3.IDReturns(555)
						pendingBuild3.ScheduleReturns(true, nil)
						pendingBuilds = []db.Build{pendingBuild1, pendingBuild2, pendingBuild3}
					})

					Context("when marking the build as scheduled fails", func() {
						BeforeEach(func() {
							pendingBuild1.ScheduleReturns(false, disaster)
						})

						It("returns the error", func() {
							Expect(tryStartErr).To(Equal(disaster))
						})

						It("marked the right build as scheduled", func() {
							Expect(pendingBuild1.ScheduleCallCount()).To(Equal(1))
						})
					})

					Context("when someone else already scheduled the build", func() {
						BeforeEach(func() {
							pendingBuild1.ScheduleReturns(false, nil)
						})

						It("doesn't return an error", func() {
							Expect(tryStartErr).NotTo(HaveOccurred())
						})

						It("doesn't try to use inputs for build", func() {
							Expect(pendingBuild1.UseInputsCallCount()).To(BeZero())
						})
					})

					Context("when marking the build as scheduled succeeds", func() {
						BeforeEach(func() {
							pendingBuild1.ScheduleReturns(true, nil)
						})

						Context("when using inputs for build fails", func() {
							BeforeEach(func() {
								pendingBuild1.UseInputsReturns(disaster)
							})

							It("returns the error", func() {
								Expect(tryStartErr).To(Equal(disaster))
							})

							It("used the right inputs for the right build", func() {
								Expect(pendingBuild1.UseInputsCallCount()).To(Equal(1))
								actualInputs := pendingBuild1.UseInputsArgsForCall(0)
								Expect(actualInputs).To(Equal([]db.BuildInput{{Name: "some-input"}}))
							})
						})

						Context("when using inputs for build succeeds", func() {
							BeforeEach(func() {
								pendingBuild1.UseInputsReturns(nil)
							})

							Context("when creating the build plan fails", func() {
								BeforeEach(func() {
									fakeFactory.CreateReturns(atc.Plan{}, disaster)
								})

								It("stops creating builds for job", func() {
									Expect(fakeFactory.CreateCallCount()).To(Equal(1))
									actualJobConfig, actualResourceConfigs, actualResourceTypes, actualBuildInputs := fakeFactory.CreateArgsForCall(0)
									Expect(actualJobConfig).To(Equal(atc.JobConfig{Name: "some-job"}))
									Expect(actualResourceConfigs).To(Equal(atc.ResourceConfigs{{Name: "some-resource"}}))
									Expect(actualResourceTypes).To(Equal(versionedResourceTypes))
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
										Expect(actualStatus).To(Equal(db.BuildStatusErrored))
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
									fakeFactory.CreateReturns(atc.Plan{Task: &atc.TaskPlan{ConfigPath: "some-task-1.yml"}}, nil)
									fakeEngine.CreateBuildReturns(new(enginefakes.FakeBuild), nil)
								})

								It("creates build plans for all builds", func() {
									Expect(fakeFactory.CreateCallCount()).To(Equal(3))
									actualJobConfig, actualResourceConfigs, actualResourceTypes, actualBuildInputs := fakeFactory.CreateArgsForCall(0)
									Expect(actualJobConfig).To(Equal(atc.JobConfig{Name: "some-job"}))
									Expect(actualResourceConfigs).To(Equal(atc.ResourceConfigs{{Name: "some-resource"}}))
									Expect(actualResourceTypes).To(Equal(versionedResourceTypes))
									Expect(actualBuildInputs).To(Equal([]db.BuildInput{{Name: "some-input"}}))

									actualJobConfig, actualResourceConfigs, actualResourceTypes, actualBuildInputs = fakeFactory.CreateArgsForCall(1)
									Expect(actualJobConfig).To(Equal(atc.JobConfig{Name: "some-job"}))
									Expect(actualResourceConfigs).To(Equal(atc.ResourceConfigs{{Name: "some-resource"}}))
									Expect(actualResourceTypes).To(Equal(versionedResourceTypes))
									Expect(actualBuildInputs).To(Equal([]db.BuildInput{{Name: "some-input"}}))

									actualJobConfig, actualResourceConfigs, actualResourceTypes, actualBuildInputs = fakeFactory.CreateArgsForCall(2)
									Expect(actualJobConfig).To(Equal(atc.JobConfig{Name: "some-job"}))
									Expect(actualResourceConfigs).To(Equal(atc.ResourceConfigs{{Name: "some-resource"}}))
									Expect(actualResourceTypes).To(Equal(versionedResourceTypes))
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

									itUpdatedMaxInFlightForAllBuilds()

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
										Expect(actualPlan).To(Equal(atc.Plan{Task: &atc.TaskPlan{ConfigPath: "some-task-1.yml"}}))
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
						itUpdatedMaxInFlightForTheFirstBuild()
					})

					Context("when max in flight is reached", func() {
						BeforeEach(func() {
							fakeUpdater.UpdateMaxInFlightReachedReturns(true, nil)
						})

						itDoesntReturnAnErrorOrMarkTheBuildAsScheduled()
					})

					Context("when getting the next build inputs fails", func() {
						BeforeEach(func() {
							job.GetNextBuildInputsReturns(nil, false, disaster)
						})

						itReturnsTheError()
						itUpdatedMaxInFlightForTheFirstBuild()
					})

					Context("when there are no next build inputs", func() {
						BeforeEach(func() {
							job.GetNextBuildInputsReturns(nil, false, nil)
						})

						itDoesntReturnAnErrorOrMarkTheBuildAsScheduled()
						itUpdatedMaxInFlightForTheFirstBuild()
					})

					Context("when checking if the pipeline is paused fails", func() {
						BeforeEach(func() {
							fakePipeline.CheckPausedReturns(false, disaster)
						})

						itReturnsTheError()
						itUpdatedMaxInFlightForTheFirstBuild()
					})

					Context("when the pipeline is paused", func() {
						BeforeEach(func() {
							fakePipeline.CheckPausedReturns(true, nil)
						})

						itDoesntReturnAnErrorOrMarkTheBuildAsScheduled()
						itUpdatedMaxInFlightForTheFirstBuild()
					})

					Context("when the job is paused", func() {
						BeforeEach(func() {
							job.PausedReturns(true)
						})

						itDoesntReturnAnErrorOrMarkTheBuildAsScheduled()
						itUpdatedMaxInFlightForTheFirstBuild()
					})
				})
			})
		})
	})
})
