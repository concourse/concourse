package scheduler_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/scheduler"
	"github.com/concourse/concourse/atc/scheduler/algorithm"
	"github.com/concourse/concourse/atc/scheduler/schedulerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildStarter", func() {
	var (
		fakePipeline  *dbfakes.FakePipeline
		fakeFactory   *schedulerfakes.FakeBuildFactory
		pendingBuilds []db.Build
		fakeAlgorithm *schedulerfakes.FakeAlgorithm

		buildStarter scheduler.BuildStarter

		disaster error
	)

	BeforeEach(func() {
		fakePipeline = new(dbfakes.FakePipeline)
		fakeFactory = new(schedulerfakes.FakeBuildFactory)
		fakeAlgorithm = new(schedulerfakes.FakeAlgorithm)

		buildStarter = scheduler.NewBuildStarter(fakeFactory, fakeAlgorithm)

		disaster = errors.New("bad thing")
	})

	Describe("TryStartPendingBuildsForJob", func() {
		var tryStartErr error
		var createdBuild *dbfakes.FakeBuild
		var job *dbfakes.FakeJob
		var resource *dbfakes.FakeResource
		var resources db.Resources
		var versionedResourceTypes atc.VersionedResourceTypes
		var relatedJobs algorithm.NameToIDMap

		BeforeEach(func() {
			versionedResourceTypes = atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{Name: "some-resource-type"},
					Version:      atc.Version{"some": "version"},
				},
			}

			resource = new(dbfakes.FakeResource)
			resource.NameReturns("some-resource")
		})

		Context("when pending builds are successfully fetched", func() {
			BeforeEach(func() {
				createdBuild = new(dbfakes.FakeBuild)
				createdBuild.IDReturns(66)
				createdBuild.NameReturns("some-build")

				pendingBuilds = []db.Build{createdBuild}

				job = new(dbfakes.FakeJob)
				job.GetPendingBuildsReturns(pendingBuilds, nil)
				job.NameReturns("some-job")
				job.IDReturns(1)
				job.ConfigReturns(atc.JobConfig{Plan: atc.PlanSequence{{Get: "input-1", Resource: "some-resource"}, {Get: "input-2", Resource: "some-resource"}}})

				relatedJobs = algorithm.NameToIDMap{"some-job": 1}

				fakePipeline.CheckPausedReturns(false, nil)
			})

			Context("when one pending build is aborted before start", func() {
				var abortedBuild *dbfakes.FakeBuild

				BeforeEach(func() {
					abortedBuild = new(dbfakes.FakeBuild)
					abortedBuild.IDReturns(42)
					abortedBuild.IsAbortedReturns(true)

					// make sure pending build can be started after another pending build is aborted
					pendingBuilds = append([]db.Build{abortedBuild}, pendingBuilds...)
					job.GetPendingBuildsReturns(pendingBuilds, nil)
					resources = db.Resources{resource}
				})

				JustBeforeEach(func() {
					tryStartErr = buildStarter.TryStartPendingBuildsForJob(
						lagertest.NewTestLogger("test"),
						fakePipeline,
						job,
						resources,
						relatedJobs,
					)
				})

				It("won't try to start the aborted pending build", func() {
					Expect(abortedBuild.FinishCallCount()).To(Equal(1))
				})

				It("will try to start the next non aborted pending build", func() {
					Expect(job.ScheduleBuildCallCount()).To(Equal(1))
					actualBuild := job.ScheduleBuildArgsForCall(0)
					Expect(actualBuild.Name()).To(Equal(createdBuild.Name()))
				})

				It("returns without error", func() {
					Expect(tryStartErr).NotTo(HaveOccurred())
				})
			})

			Context("when manually triggered", func() {
				BeforeEach(func() {
					createdBuild.IsManuallyTriggeredReturns(true)

					resources = db.Resources{resource}
				})

				JustBeforeEach(func() {
					tryStartErr = buildStarter.TryStartPendingBuildsForJob(
						lagertest.NewTestLogger("test"),
						fakePipeline,
						job,
						resources,
						relatedJobs,
					)
				})

				It("tries to schedule the build", func() {
					Expect(job.ScheduleBuildCallCount()).To(Equal(1))
					actualBuild := job.ScheduleBuildArgsForCall(0)
					Expect(actualBuild.Name()).To(Equal(createdBuild.Name()))
				})

				Context("when the build not scheduled", func() {
					BeforeEach(func() {
						job.ScheduleBuildReturns(false, nil)
					})

					It("does not start the build", func() {
						Expect(createdBuild.StartCallCount()).To(BeZero())
					})
				})

				Context("when the build is successfully scheduled", func() {
					BeforeEach(func() {
						job.ScheduleBuildReturns(true, nil)
					})

					Context("when some of the resources are checked before build create time", func() {
						BeforeEach(func() {
							createdBuild.IsNewerThanLastCheckOfReturns(true)
						})

						It("does not save the next input mapping", func() {
							Expect(fakeAlgorithm.ComputeCallCount()).To(BeZero())
						})

						It("does not start the build", func() {
							Expect(createdBuild.StartCallCount()).To(BeZero())
						})

						It("returns without error", func() {
							Expect(tryStartErr).NotTo(HaveOccurred())
						})
					})

					Context("when all resources are checked after build create time or pinned", func() {
						BeforeEach(func() {
							fakeDBResourceType := new(dbfakes.FakeResourceType)
							fakeDBResourceType.NameReturns("fake-resource-type")
							fakeDBResourceType.TypeReturns("fake")
							fakeDBResourceType.SourceReturns(atc.Source{"im": "fake"})
							fakeDBResourceType.PrivilegedReturns(true)
							fakeDBResourceType.VersionReturns(atc.Version{"version": "1.2.3"})

							fakePipeline.ResourceTypesReturns(db.ResourceTypes{fakeDBResourceType}, nil)

							job.ConfigReturns(atc.JobConfig{Plan: atc.PlanSequence{{Get: "input-1", Resource: "some-resource"}, {Get: "input-2", Resource: "other-resource"}}})

							createdBuild.IsNewerThanLastCheckOfReturns(false)

							otherResource := new(dbfakes.FakeResource)
							otherResource.IDReturns(25)
							otherResource.NameReturns("other-resource")
							otherResource.CurrentPinnedVersionReturns(atc.Version{"some": "version"})
							otherResource.LastCheckEndTimeReturns(time.Now().Add(-time.Minute))

							resources = db.Resources{resource, otherResource}
						})

						It("computes a new set of versions for inputs to the build", func() {
							Expect(fakeAlgorithm.ComputeCallCount()).To(Equal(1))
						})

						Context("when computing the next inputs fails", func() {
							BeforeEach(func() {
								fakeAlgorithm.ComputeReturns(nil, false, false, disaster)
							})

							It("computes the next inputs for the right job and versions", func() {
								Expect(fakeAlgorithm.ComputeCallCount()).To(Equal(1))
								actualJob, _, actualRelatedJobs := fakeAlgorithm.ComputeArgsForCall(0)
								Expect(actualJob.Name()).To(Equal(job.Name()))
								Expect(actualRelatedJobs).To(Equal(relatedJobs))
							})
						})

						Context("when computing the next inputs succeeds", func() {
							var expectedInputMapping db.InputMapping

							BeforeEach(func() {
								expectedInputMapping = map[string]db.InputResult{
									"input-1": db.InputResult{
										Input: &db.AlgorithmInput{
											AlgorithmVersion: db.AlgorithmVersion{
												ResourceID: 1,
												Version:    db.ResourceVersion("1"),
											},
											FirstOccurrence: true,
										},
									},
								}

								fakeAlgorithm.ComputeReturns(expectedInputMapping, true, false, nil)
							})

							Context("when the algorithm can run again", func() {
								BeforeEach(func() {
									fakeAlgorithm.ComputeReturns(expectedInputMapping, true, true, nil)
								})

								It("requests schedule on the pipeline", func() {
									Expect(fakePipeline.RequestScheduleCallCount()).To(Equal(1))
								})
							})

							Context("when the algorithm can not run again", func() {
								BeforeEach(func() {
									fakeAlgorithm.ComputeReturns(expectedInputMapping, true, false, nil)
								})

								It("does not requests schedule on the pipeline", func() {
									Expect(fakePipeline.RequestScheduleCallCount()).To(Equal(0))
								})
							})

							It("saves the next input mapping", func() {
								Expect(job.SaveNextInputMappingCallCount()).To(Equal(1))
							})

							Context("when saving the next input mapping fails", func() {
								BeforeEach(func() {
									job.SaveNextInputMappingReturns(disaster)
								})

								It("saves the next input mapping with the right inputs", func() {
									actualInputMapping, resolved := job.SaveNextInputMappingArgsForCall(0)
									Expect(actualInputMapping).To(Equal(expectedInputMapping))
									Expect(resolved).To(BeTrue())
								})
							})

							Context("when saving the next input mapping succeeds", func() {
								BeforeEach(func() {
									job.SaveNextInputMappingReturns(nil)
								})

								It("saved the next input mapping and adopts the inputs and pipes", func() {
									Expect(createdBuild.AdoptInputsAndPipesCallCount()).To(Equal(1))
									Expect(tryStartErr).NotTo(HaveOccurred())
								})
							})

							Context("when creating a build plan", func() {
								BeforeEach(func() {
									createdBuild.AdoptInputsAndPipesReturns([]db.BuildInput{}, true, nil)
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

			Context("when not manually triggered", func() {
				var pendingBuild1 *dbfakes.FakeBuild
				var pendingBuild2 *dbfakes.FakeBuild
				var pendingBuild3 *dbfakes.FakeBuild

				BeforeEach(func() {
					job.NameReturns("some-job")
					job.IDReturns(1)
					job.ConfigReturns(atc.JobConfig{Name: "some-job"})
					createdBuild.IsManuallyTriggeredReturns(false)

					relatedJobs = algorithm.NameToIDMap{"some-job": 1}

					fakeDBResourceType := new(dbfakes.FakeResourceType)
					fakeDBResourceType.NameReturns("some-resource-type")
					fakeDBResourceType.VersionReturns(atc.Version{"some": "version"})

					fakePipeline.ResourceTypesReturns(db.ResourceTypes{fakeDBResourceType}, nil)
				})

				JustBeforeEach(func() {
					tryStartErr = buildStarter.TryStartPendingBuildsForJob(
						lagertest.NewTestLogger("test"),
						fakePipeline,
						job,
						db.Resources{resource},
						relatedJobs,
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
						Expect(job.ScheduleBuildCallCount()).To(BeZero())
					})
				}

				itScheduledAllBuilds := func() {
					It("scheduled all the pending builds", func() {
						Expect(job.ScheduleBuildCallCount()).To(Equal(3))
						actualBuild := job.ScheduleBuildArgsForCall(0)
						Expect(actualBuild.ID()).To(Equal(pendingBuild1.ID()))

						actualBuild = job.ScheduleBuildArgsForCall(1)
						Expect(actualBuild.ID()).To(Equal(pendingBuild2.ID()))

						actualBuild = job.ScheduleBuildArgsForCall(2)
						Expect(actualBuild.ID()).To(Equal(pendingBuild3.ID()))
					})
				}

				itAttemptedToScheduleFirstBuild := func() {
					It("tried to schedule the first pending build", func() {
						Expect(job.ScheduleBuildCallCount()).To(Equal(1))
						actualBuild := job.ScheduleBuildArgsForCall(0)
						Expect(actualBuild.ID()).To(Equal(pendingBuild1.ID()))
					})
				}

				itDidNotAttemptToScheduleAnyBuilds := func() {
					It("did not try to schedule any builds", func() {
						Expect(job.ScheduleBuildCallCount()).To(Equal(0))
					})
				}

				Context("when the stars align", func() {
					BeforeEach(func() {
						job.PausedReturns(false)
						job.ScheduleBuildReturns(true, nil)
						fakePipeline.PausedReturns(false)
					})

					Context("when there are several pending builds consisting of both retrigger and normal scheduler builds", func() {
						BeforeEach(func() {
							pendingBuild1 = new(dbfakes.FakeBuild)
							pendingBuild1.IDReturns(99)
							pendingBuild1.AdoptInputsAndPipesReturns([]db.BuildInput{{Name: "some-input"}}, true, nil)
							job.ScheduleBuildReturnsOnCall(0, true, nil)
							pendingBuild2 = new(dbfakes.FakeBuild)
							pendingBuild2.IDReturns(999)
							pendingBuild2.AdoptInputsAndPipesReturns([]db.BuildInput{{Name: "some-input"}}, true, nil)
							job.ScheduleBuildReturnsOnCall(1, true, nil)
							pendingBuild3 = new(dbfakes.FakeBuild)
							pendingBuild3.IDReturns(555)
							pendingBuild3.RerunOfReturns(pendingBuild1.ID())
							pendingBuild3.AdoptRerunInputsAndPipesReturns([]db.BuildInput{{Name: "some-input"}}, true, nil)
							job.ScheduleBuildReturnsOnCall(2, true, nil)
							pendingBuilds = []db.Build{pendingBuild1, pendingBuild2, pendingBuild3}
							job.GetPendingBuildsReturns(pendingBuilds, nil)
						})

						Context("when marking the build as scheduled fails", func() {
							BeforeEach(func() {
								job.ScheduleBuildReturnsOnCall(0, false, disaster)
							})

							It("returns the error", func() {
								Expect(tryStartErr).To(Equal(disaster))
							})

							It("only tried to schedule one pending build", func() {
								Expect(job.ScheduleBuildCallCount()).To(Equal(1))
							})
						})

						Context("when the build was not able to be scheduled", func() {
							BeforeEach(func() {
								job.ScheduleBuildReturnsOnCall(1, false, nil)
							})

							It("doesn't return an error", func() {
								Expect(tryStartErr).NotTo(HaveOccurred())
							})

							It("doesn't try adopt build inputs and pipes for that pending build and doesn't try scheduling the next ones", func() {
								Expect(pendingBuild1.AdoptInputsAndPipesCallCount()).To(Equal(1))
								Expect(pendingBuild2.AdoptInputsAndPipesCallCount()).To(BeZero())
								Expect(pendingBuild3.AdoptRerunInputsAndPipesCallCount()).To(BeZero())
							})
						})

						Context("when the build was scheduled successfully", func() {
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
									pendingBuild1.StartReturns(true, nil)
									pendingBuild2.StartReturns(true, nil)
									pendingBuild3.StartReturns(true, nil)
								})

								It("adopts the build inputs and pipes", func() {
									Expect(pendingBuild1.AdoptInputsAndPipesCallCount()).To(Equal(1))
									Expect(pendingBuild1.AdoptRerunInputsAndPipesCallCount()).To(BeZero())

									Expect(pendingBuild2.AdoptInputsAndPipesCallCount()).To(Equal(1))
									Expect(pendingBuild2.AdoptRerunInputsAndPipesCallCount()).To(BeZero())

									Expect(pendingBuild3.AdoptInputsAndPipesCallCount()).To(BeZero())
									Expect(pendingBuild3.AdoptRerunInputsAndPipesCallCount()).To(Equal(1))
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

								Context("when starting the build fails", func() {
									BeforeEach(func() {
										pendingBuild1.StartReturns(false, disaster)
									})

									It("doesn't return an error", func() {
										Expect(tryStartErr).NotTo(HaveOccurred())
									})
								})

								Context("when starting the build returns false", func() {
									BeforeEach(func() {
										pendingBuild1.StartReturns(false, nil)
									})

									It("doesn't return an error", func() {
										Expect(tryStartErr).NotTo(HaveOccurred())
									})

									It("finishes the build with aborted status", func() {
										Expect(pendingBuild1.FinishCallCount()).To(Equal(1))
										Expect(pendingBuild1.FinishArgsForCall(0)).To(Equal(db.BuildStatusAborted))
									})
								})

								Context("when starting the builds returns true", func() {
									BeforeEach(func() {
										pendingBuild1.StartReturns(true, nil)
										pendingBuild2.StartReturns(true, nil)
										pendingBuild3.StartReturns(true, nil)
									})

									It("doesn't return an error", func() {
										Expect(tryStartErr).NotTo(HaveOccurred())
									})

									itScheduledAllBuilds()

									It("starts the build with the right plan", func() {
										Expect(pendingBuild1.StartCallCount()).To(Equal(1))
										Expect(pendingBuild1.StartArgsForCall(0)).To(Equal(atc.Plan{Task: &atc.TaskPlan{ConfigPath: "some-task-1.yml"}}))

										Expect(pendingBuild2.StartCallCount()).To(Equal(1))
										Expect(pendingBuild2.StartArgsForCall(0)).To(Equal(atc.Plan{Task: &atc.TaskPlan{ConfigPath: "some-task-1.yml"}}))

										Expect(pendingBuild3.StartCallCount()).To(Equal(1))
										Expect(pendingBuild3.StartArgsForCall(0)).To(Equal(atc.Plan{Task: &atc.TaskPlan{ConfigPath: "some-task-1.yml"}}))
									})
								})
							})
						})

						Context("when adopting the inputs and pipes fails", func() {
							BeforeEach(func() {
								pendingBuild1.AdoptInputsAndPipesReturns(nil, false, disaster)
							})

							itReturnsTheError()
							itAttemptedToScheduleFirstBuild()
						})

						Context("when there are no next build inputs", func() {
							BeforeEach(func() {
								pendingBuild1.AdoptInputsAndPipesReturns(nil, false, nil)
							})

							It("doesn't return an error", func() {
								Expect(tryStartErr).NotTo(HaveOccurred())
							})

							It("does not start the build", func() {
								Expect(createdBuild.StartCallCount()).To(BeZero())
							})

							itAttemptedToScheduleFirstBuild()
						})

						Context("when checking if the pipeline is paused fails", func() {
							BeforeEach(func() {
								fakePipeline.CheckPausedReturns(false, disaster)
							})

							itReturnsTheError()
							itDidNotAttemptToScheduleAnyBuilds()
						})

						Context("when the pipeline is paused", func() {
							BeforeEach(func() {
								fakePipeline.CheckPausedReturns(true, nil)
							})

							itDoesntReturnAnErrorOrMarkTheBuildAsScheduled()
							itDidNotAttemptToScheduleAnyBuilds()
						})

						Context("when the job is paused", func() {
							BeforeEach(func() {
								job.PausedReturns(true)
							})

							itDoesntReturnAnErrorOrMarkTheBuildAsScheduled()
							itDidNotAttemptToScheduleAnyBuilds()
						})

						Context("when fetching pending builds fail", func() {
							BeforeEach(func() {
								job.GetPendingBuildsReturns(nil, disaster)
							})

							itReturnsTheError()
						})
					})
				})
			})
		})
	})
})
