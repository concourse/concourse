package scheduler_test

import (
	"errors"
	"fmt"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/scheduler"
	"github.com/concourse/concourse/atc/scheduler/schedulerfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildStarter", func() {
	var (
		fakePipeline  *dbfakes.FakePipeline
		fakePlanner   *schedulerfakes.FakeBuildPlanner
		pendingBuilds []db.Build
		fakeAlgorithm *schedulerfakes.FakeAlgorithm

		buildStarter scheduler.BuildStarter

		jobInputs db.InputConfigs

		disaster error
	)

	BeforeEach(func() {
		fakePipeline = new(dbfakes.FakePipeline)
		fakePlanner = new(schedulerfakes.FakeBuildPlanner)
		fakeAlgorithm = new(schedulerfakes.FakeAlgorithm)

		buildStarter = scheduler.NewBuildStarter(fakePlanner, fakeAlgorithm)

		disaster = errors.New("bad thing")
	})

	Describe("TryStartPendingBuildsForJob", func() {
		var tryStartErr error
		var needsReschedule bool
		var createdBuild *dbfakes.FakeBuild
		var job *dbfakes.FakeJob
		var resources db.SchedulerResources
		var resourceTypes atc.ResourceTypes
		var prototypes atc.Prototypes

		BeforeEach(func() {
			resourceTypes = atc.ResourceTypes{
				atc.ResourceType{Name: "some-resource-type"},
			}

			resources = db.SchedulerResources{
				{
					Name: "some-resource",
				},
			}

			prototypes = atc.Prototypes{
				{
					Name: "some-prototype",
				},
			}
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
				job.ConfigReturns(atc.JobConfig{
					PlanSequence: []atc.Step{
						{
							Config: &atc.GetStep{
								Name:     "input-1",
								Resource: "some-resource",
							},
						}, {
							Config: &atc.GetStep{
								Name:     "input-2",
								Resource: "some-resource",
							},
						},
					},
				}, nil)

				jobInputs = db.InputConfigs{
					{
						Name:       "input-1",
						ResourceID: 1,
					},
					{
						Name:       "input-2",
						ResourceID: 1,
					},
				}
			})

			Context("when one pending build is aborted before start", func() {
				var abortedBuild *dbfakes.FakeBuild

				BeforeEach(func() {
					abortedBuild = new(dbfakes.FakeBuild)
					abortedBuild.IDReturns(42)
					abortedBuild.IsAbortedReturns(true)
					abortedBuild.FinishReturns(nil)
				})

				JustBeforeEach(func() {
					needsReschedule, tryStartErr = buildStarter.TryStartPendingBuildsForJob(
						lagertest.NewTestLogger("test"),
						db.SchedulerJob{
							Job:           job,
							Resources:     resources,
							ResourceTypes: resourceTypes,
							Prototypes:    prototypes,
						},
						jobInputs,
					)
				})

				Context("when there is one aborted build", func() {
					BeforeEach(func() {
						pendingBuilds = []db.Build{abortedBuild}
						job.GetPendingBuildsReturns(pendingBuilds, nil)
					})

					It("won't try to start the aborted pending build", func() {
						Expect(abortedBuild.FinishCallCount()).To(Equal(1))
					})

					It("returns without error", func() {
						Expect(tryStartErr).NotTo(HaveOccurred())
						Expect(needsReschedule).To(BeFalse())
					})

					Context("when finishing the aborted build fails", func() {
						BeforeEach(func() {
							abortedBuild.FinishReturns(disaster)
						})

						It("returns an error", func() {
							Expect(tryStartErr).To(Equal(fmt.Errorf("finish aborted build: %w", disaster)))
							Expect(needsReschedule).To(BeFalse())
						})
					})
				})

				Context("when there is multiple pending builds after the aborted build", func() {
					BeforeEach(func() {
						// make sure pending build can be started after another pending build is aborted
						pendingBuilds = append([]db.Build{abortedBuild}, pendingBuilds...)
						job.GetPendingBuildsReturns(pendingBuilds, nil)
					})

					It("will try to start the next non aborted pending build", func() {
						Expect(job.ScheduleBuildCallCount()).To(Equal(1))
						actualBuild := job.ScheduleBuildArgsForCall(0)
						Expect(actualBuild.Name()).To(Equal(createdBuild.Name()))
					})
				})
			})

			Context("when manually triggered", func() {
				BeforeEach(func() {
					createdBuild.IsManuallyTriggeredReturns(true)

					resources = db.SchedulerResources{
						{
							Name: "some-resource",
						},
					}
				})

				JustBeforeEach(func() {
					needsReschedule, tryStartErr = buildStarter.TryStartPendingBuildsForJob(
						lagertest.NewTestLogger("test"),
						db.SchedulerJob{
							Job:       job,
							Resources: resources,
						},
						jobInputs,
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

					It("does not start the build and needs to be rescheduled", func() {
						Expect(createdBuild.StartCallCount()).To(BeZero())
						Expect(tryStartErr).ToNot(HaveOccurred())
						Expect(needsReschedule).To(BeTrue())
					})
				})

				Context("when scheduling the build fails", func() {
					BeforeEach(func() {
						job.ScheduleBuildReturns(false, disaster)
					})

					It("returns the error", func() {
						Expect(tryStartErr).To(Equal(fmt.Errorf("schedule build: %w", disaster)))
						Expect(needsReschedule).To(BeFalse())
					})
				})

				Context("when the build is successfully scheduled", func() {
					BeforeEach(func() {
						job.ScheduleBuildReturns(true, nil)
					})

					Context("when checking if resources have been checked fails", func() {
						BeforeEach(func() {
							createdBuild.ResourcesCheckedReturns(false, disaster)
						})

						It("returns the error", func() {
							Expect(tryStartErr).To(Equal(fmt.Errorf("ready to determine inputs: %w", disaster)))
							Expect(needsReschedule).To(BeFalse())
						})
					})

					Context("when some of the resources are checked before build create time", func() {
						BeforeEach(func() {
							createdBuild.ResourcesCheckedReturns(false, nil)
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

						It("retries to schedule", func() {
							Expect(needsReschedule).To(BeTrue())
						})
					})

					Context("when all resources are checked after build create time or pinned", func() {
						BeforeEach(func() {
							createdBuild.ResourcesCheckedReturns(true, nil)
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
								_, actualJob, actualInputs := fakeAlgorithm.ComputeArgsForCall(0)
								Expect(actualJob).To(Equal(
									db.SchedulerJob{
										Job:       job,
										Resources: resources,
									}))
								Expect(actualInputs).To(Equal(jobInputs))
							})

							It("returns the error and retries to schedule", func() {
								Expect(tryStartErr).To(Equal(fmt.Errorf("get build inputs: %w", fmt.Errorf("compute inputs: %w", disaster))))
								Expect(needsReschedule).To(BeFalse())
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

								It("requests schedule on the job", func() {
									Expect(job.RequestScheduleCallCount()).To(Equal(1))
								})

								Context("when requesting schedule fails", func() {
									BeforeEach(func() {
										job.RequestScheduleReturns(disaster)
									})

									It("returns the error and retries to schedule", func() {
										Expect(tryStartErr).To(Equal(fmt.Errorf("get build inputs: %w", fmt.Errorf("request schedule: %w", disaster))))
										Expect(needsReschedule).To(BeFalse())
									})
								})
							})

							Context("when the algorithm can not run again", func() {
								BeforeEach(func() {
									fakeAlgorithm.ComputeReturns(expectedInputMapping, true, false, nil)
								})

								It("does not requests schedule on the job", func() {
									Expect(job.RequestScheduleCallCount()).To(Equal(0))
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

								It("returns the error and retries to schedule", func() {
									Expect(tryStartErr).To(Equal(fmt.Errorf("get build inputs: %w", fmt.Errorf("save next input mapping: %w", disaster))))
									Expect(needsReschedule).To(BeFalse())
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

							Context("when adopting inputs and pipes succeeds", func() {
								BeforeEach(func() {
									createdBuild.AdoptInputsAndPipesReturns([]db.BuildInput{}, true, nil)
								})

								It("tries to fetch the job config", func() {
									Expect(job.ConfigCallCount()).To(Equal(1))
								})

								It("creates the build plan with manually triggered", func() {
									_, _, _, _, _, actualManuallyTriggered := fakePlanner.CreateArgsForCall(0)
									Expect(actualManuallyTriggered).To(Equal(true))
								})
							})

							Context("when adopting inputs and pipes fails", func() {
								BeforeEach(func() {
									createdBuild.AdoptInputsAndPipesReturns(nil, false, errors.New("error"))
								})

								It("returns an error and retries to schedule", func() {
									Expect(tryStartErr).To(HaveOccurred())
									Expect(needsReschedule).To(BeFalse())
								})
							})

							Context("when adopting inputs and pipes has no satisfiable inputs", func() {
								BeforeEach(func() {
									createdBuild.AdoptInputsAndPipesReturns(nil, false, nil)
								})

								It("does not return an error and does not try to reschedule", func() {
									Expect(tryStartErr).ToNot(HaveOccurred())
									Expect(needsReschedule).To(BeFalse())
								})
							})
						})
					})
				})
			})

			Context("when not manually triggered", func() {
				var pendingBuild1 *dbfakes.FakeBuild
				var pendingBuild2 *dbfakes.FakeBuild
				var rerunBuild *dbfakes.FakeBuild

				var jobConfig = atc.JobConfig{
					Name: "some-job",
					PlanSequence: []atc.Step{
						{
							Config: &atc.GetStep{
								Name: "some-input",
							},
						},
					},
				}

				var plannedPlan = atc.Plan{
					Get: &atc.GetPlan{
						Name:     "some-input",
						Resource: "some-input",
					},
				}

				BeforeEach(func() {
					job.NameReturns("some-job")
					job.IDReturns(1)
					job.ConfigReturns(jobConfig, nil)
					createdBuild.IsManuallyTriggeredReturns(false)

					jobInputs = db.InputConfigs{}
				})

				JustBeforeEach(func() {
					needsReschedule, tryStartErr = buildStarter.TryStartPendingBuildsForJob(
						lagertest.NewTestLogger("test"),
						db.SchedulerJob{
							Job:       job,
							Resources: resources,
							ResourceTypes: atc.ResourceTypes{
								atc.ResourceType{
									Name: "some-resource-type",
								},
							},
							Prototypes: prototypes,
						},
						jobInputs,
					)
				})

				It("doesn't compute the algorithm", func() {
					Expect(fakeAlgorithm.ComputeCallCount()).To(Equal(0))
				})

				itScheduledAllBuilds := func() {
					It("scheduled all the pending builds", func() {
						Expect(job.ScheduleBuildCallCount()).To(Equal(3))
						actualBuild := job.ScheduleBuildArgsForCall(0)
						Expect(actualBuild.ID()).To(Equal(pendingBuild1.ID()))

						actualBuild = job.ScheduleBuildArgsForCall(1)
						Expect(actualBuild.ID()).To(Equal(rerunBuild.ID()))

						actualBuild = job.ScheduleBuildArgsForCall(2)
						Expect(actualBuild.ID()).To(Equal(pendingBuild2.ID()))
					})
				}

				Context("when the stars align", func() {
					BeforeEach(func() {
						job.PausedReturns(false)
						job.ScheduleBuildReturns(true, nil)
						fakePipeline.PausedReturns(false)
					})

					Context("when adopting inputs and pipes for a rerun build fails", func() {
						BeforeEach(func() {
							pendingBuild1 = new(dbfakes.FakeBuild)
							pendingBuild1.IDReturns(99)
							pendingBuild1.RerunOfReturns(1)
							pendingBuild1.AdoptRerunInputsAndPipesReturns([]db.BuildInput{{Name: "some-input"}}, false, disaster)
							job.GetPendingBuildsReturns([]db.Build{pendingBuild1}, nil)
						})

						It("returns the error and retries to schedule", func() {
							Expect(tryStartErr).To(Equal(fmt.Errorf("get build inputs: %w", fmt.Errorf("adopt rerun inputs and pipes: %w", disaster))))
							Expect(needsReschedule).To(BeFalse())
						})
					})

					Context("when adopting inputs and pipes for a rerun build has no satisfiable inputs", func() {
						BeforeEach(func() {
							pendingBuild1 = new(dbfakes.FakeBuild)
							pendingBuild1.IDReturns(99)
							pendingBuild1.RerunOfReturns(1)
							pendingBuild1.AdoptRerunInputsAndPipesReturns([]db.BuildInput{{Name: "some-input"}}, false, nil)
							job.GetPendingBuildsReturns([]db.Build{pendingBuild1}, nil)
						})

						It("returns the error and does not retry to schedule", func() {
							Expect(tryStartErr).ToNot(HaveOccurred())
							Expect(needsReschedule).To(BeFalse())
						})
					})

					Context("when adopting inputs and pipes for a normal scheduler build fails", func() {
						BeforeEach(func() {
							pendingBuild1 = new(dbfakes.FakeBuild)
							pendingBuild1.IDReturns(99)
							pendingBuild1.AdoptInputsAndPipesReturns([]db.BuildInput{{Name: "some-input"}}, false, disaster)
							job.GetPendingBuildsReturns([]db.Build{pendingBuild1}, nil)
						})

						It("returns the error and retries to schedule", func() {
							Expect(tryStartErr).To(Equal(fmt.Errorf("get build inputs: %w", fmt.Errorf("adopt inputs and pipes: %w", disaster))))
							Expect(needsReschedule).To(BeFalse())
						})
					})

					Context("when adopting inputs and pipes for a normal scheduler build has no satisfiable inputs", func() {
						BeforeEach(func() {
							pendingBuild1 = new(dbfakes.FakeBuild)
							pendingBuild1.IDReturns(99)
							pendingBuild1.AdoptInputsAndPipesReturns([]db.BuildInput{{Name: "some-input"}}, false, nil)
							job.GetPendingBuildsReturns([]db.Build{pendingBuild1}, nil)
						})

						It("returns the error and does not retry to schedule", func() {
							Expect(tryStartErr).ToNot(HaveOccurred())
							Expect(needsReschedule).To(BeFalse())
						})
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
							rerunBuild = new(dbfakes.FakeBuild)
							rerunBuild.IDReturns(555)
							rerunBuild.RerunOfReturns(pendingBuild1.ID())
							rerunBuild.AdoptRerunInputsAndPipesReturns([]db.BuildInput{{Name: "some-input"}}, true, nil)
							job.ScheduleBuildReturnsOnCall(2, true, nil)
							pendingBuilds = []db.Build{pendingBuild1, rerunBuild, pendingBuild2}
							job.GetPendingBuildsReturns(pendingBuilds, nil)
						})

						Context("when marking the build as scheduled fails", func() {
							BeforeEach(func() {
								job.ScheduleBuildReturnsOnCall(0, false, disaster)
							})

							It("returns the error", func() {
								Expect(tryStartErr).To(Equal(fmt.Errorf("schedule build: %w", disaster)))
							})

							It("only tried to schedule one pending build", func() {
								Expect(job.ScheduleBuildCallCount()).To(Equal(1))
							})
						})

						Context("when the build was not able to be scheduled", func() {
							BeforeEach(func() {
								job.ScheduleBuildReturnsOnCall(0, false, nil)
							})

							It("doesn't return an error", func() {
								Expect(tryStartErr).NotTo(HaveOccurred())
							})

							It("doesn't try adopt build inputs and pipes for that pending build and doesn't try scheduling the next ones", func() {
								Expect(pendingBuild1.AdoptInputsAndPipesCallCount()).To(BeZero())
								Expect(pendingBuild2.AdoptInputsAndPipesCallCount()).To(BeZero())
								Expect(rerunBuild.AdoptRerunInputsAndPipesCallCount()).To(BeZero())
							})
						})

						Context("when the build was scheduled successfully", func() {
							Context("when the resource types are successfully fetched", func() {
								Context("when creating the build plan fails for the rerun build and the scheduler builds", func() {
									BeforeEach(func() {
										fakePlanner.CreateReturns(atc.Plan{}, disaster)
									})

									It("keeps going after failing to create", func() {
										Expect(fakePlanner.CreateCallCount()).To(Equal(3))

										Expect(rerunBuild.FinishCallCount()).To(Equal(1))
										Expect(pendingBuild1.FinishCallCount()).To(Equal(1))
										Expect(pendingBuild2.FinishCallCount()).To(Equal(1))
									})

									Context("when marking the build as errored fails", func() {
										BeforeEach(func() {
											pendingBuild1.FinishReturns(disaster)
										})

										It("returns an error", func() {
											Expect(tryStartErr).To(Equal(fmt.Errorf("finish build: %w", disaster)))
											Expect(needsReschedule).To(BeFalse())
										})

										It("does not start the other pending build", func() {
											Expect(pendingBuild2.StartCallCount()).To(Equal(0))
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

										It("does not start the other builds", func() {
											Expect(pendingBuild2.StartCallCount()).To(Equal(0))
										})

										It("doesn't return an error", func() {
											Expect(tryStartErr).NotTo(HaveOccurred())
											Expect(needsReschedule).To(BeFalse())
										})
									})
								})

								Context("when creating the build plan succeeds", func() {
									BeforeEach(func() {
										fakePlanner.CreateReturns(plannedPlan, nil)
										pendingBuild1.StartReturns(true, nil)
										pendingBuild2.StartReturns(true, nil)
										rerunBuild.StartReturns(true, nil)
									})

									It("adopts the build inputs and pipes", func() {
										Expect(pendingBuild1.AdoptInputsAndPipesCallCount()).To(Equal(1))
										Expect(pendingBuild1.AdoptRerunInputsAndPipesCallCount()).To(BeZero())

										Expect(pendingBuild2.AdoptInputsAndPipesCallCount()).To(Equal(1))
										Expect(pendingBuild2.AdoptRerunInputsAndPipesCallCount()).To(BeZero())

										Expect(rerunBuild.AdoptInputsAndPipesCallCount()).To(BeZero())
										Expect(rerunBuild.AdoptRerunInputsAndPipesCallCount()).To(Equal(1))
									})

									It("creates build plans for all builds", func() {
										Expect(fakePlanner.CreateCallCount()).To(Equal(3))

										actualPlanConfig, actualResourceConfigs, actualResourceTypes, actualPrototypes, actualBuildInputs, actualManuallyTriggered := fakePlanner.CreateArgsForCall(0)
										Expect(actualPlanConfig).To(Equal(&atc.DoStep{Steps: jobConfig.PlanSequence}))
										Expect(actualResourceConfigs).To(Equal(db.SchedulerResources{{Name: "some-resource"}}))
										Expect(actualResourceTypes).To(Equal(resourceTypes))
										Expect(actualPrototypes).To(Equal(prototypes))
										Expect(actualBuildInputs).To(Equal([]db.BuildInput{{Name: "some-input"}}))
										Expect(actualManuallyTriggered).To(Equal(false))

										actualPlanConfig, actualResourceConfigs, actualResourceTypes, actualPrototypes, actualBuildInputs, actualManuallyTriggered = fakePlanner.CreateArgsForCall(1)
										Expect(actualPlanConfig).To(Equal(&atc.DoStep{Steps: jobConfig.PlanSequence}))
										Expect(actualResourceConfigs).To(Equal(db.SchedulerResources{{Name: "some-resource"}}))
										Expect(actualResourceTypes).To(Equal(resourceTypes))
										Expect(actualPrototypes).To(Equal(prototypes))
										Expect(actualBuildInputs).To(Equal([]db.BuildInput{{Name: "some-input"}}))
										Expect(actualManuallyTriggered).To(Equal(false))

										actualPlanConfig, actualResourceConfigs, actualResourceTypes, actualPrototypes, actualBuildInputs, actualManuallyTriggered = fakePlanner.CreateArgsForCall(2)
										Expect(actualPlanConfig).To(Equal(&atc.DoStep{Steps: jobConfig.PlanSequence}))
										Expect(actualResourceConfigs).To(Equal(db.SchedulerResources{{Name: "some-resource"}}))
										Expect(actualResourceTypes).To(Equal(resourceTypes))
										Expect(actualPrototypes).To(Equal(prototypes))
										Expect(actualBuildInputs).To(Equal([]db.BuildInput{{Name: "some-input"}}))
										Expect(actualManuallyTriggered).To(Equal(false))
									})

									Context("when starting the build fails", func() {
										BeforeEach(func() {
											pendingBuild1.StartReturns(false, disaster)
										})

										It("returns the error", func() {
											Expect(tryStartErr).To(Equal(fmt.Errorf("start build: %w", disaster)))
											Expect(needsReschedule).To(BeFalse())
										})

										It("does not start the other builds", func() {
											Expect(pendingBuild2.StartCallCount()).To(Equal(0))
										})
									})

									Context("when starting the build returns false", func() {
										BeforeEach(func() {
											pendingBuild1.StartReturns(false, nil)
										})

										It("doesn't return an error", func() {
											Expect(tryStartErr).NotTo(HaveOccurred())
											Expect(needsReschedule).To(BeFalse())
										})

										It("starts the other builds", func() {
											Expect(pendingBuild2.StartCallCount()).To(Equal(1))
										})

										It("finishes the build with aborted status", func() {
											Expect(pendingBuild1.FinishCallCount()).To(Equal(1))
											Expect(pendingBuild1.FinishArgsForCall(0)).To(Equal(db.BuildStatusAborted))
										})

										Context("when marking the build as errored fails", func() {
											BeforeEach(func() {
												pendingBuild1.FinishReturns(disaster)
											})

											It("returns an error", func() {
												Expect(tryStartErr).To(Equal(fmt.Errorf("finish build: %w", disaster)))
												Expect(needsReschedule).To(BeFalse())
											})

											It("does not start the other builds", func() {
												Expect(pendingBuild2.StartCallCount()).To(Equal(0))
											})

											It("marked the right build as errored", func() {
												Expect(pendingBuild1.FinishCallCount()).To(Equal(1))
												actualStatus := pendingBuild1.FinishArgsForCall(0)
												Expect(actualStatus).To(Equal(db.BuildStatusAborted))
											})
										})

										Context("when marking the build as errored succeeds", func() {
											BeforeEach(func() {
												pendingBuild1.FinishReturns(nil)
											})

											It("doesn't return an error", func() {
												Expect(tryStartErr).NotTo(HaveOccurred())
												Expect(needsReschedule).To(BeFalse())
											})

											It("starts the other builds", func() {
												Expect(pendingBuild2.StartCallCount()).To(Equal(1))
											})
										})
									})

									Context("when starting the builds returns true", func() {
										BeforeEach(func() {
											pendingBuild1.StartReturns(true, nil)
											pendingBuild2.StartReturns(true, nil)
											rerunBuild.StartReturns(true, nil)
										})

										It("doesn't return an error", func() {
											Expect(tryStartErr).NotTo(HaveOccurred())
											Expect(needsReschedule).To(BeFalse())
										})

										itScheduledAllBuilds()

										It("starts the build with the right plan", func() {
											Expect(pendingBuild1.StartCallCount()).To(Equal(1))
											Expect(pendingBuild1.StartArgsForCall(0)).To(Equal(plannedPlan))

											Expect(pendingBuild2.StartCallCount()).To(Equal(1))
											Expect(pendingBuild2.StartArgsForCall(0)).To(Equal(plannedPlan))

											Expect(rerunBuild.StartCallCount()).To(Equal(1))
											Expect(rerunBuild.StartArgsForCall(0)).To(Equal(plannedPlan))
										})
									})
								})
							})
						})

						Context("when adopting the inputs and pipes fails", func() {
							BeforeEach(func() {
								pendingBuild1.AdoptInputsAndPipesReturns(nil, false, disaster)
							})

							It("returns the error", func() {
								Expect(tryStartErr).To(Equal(fmt.Errorf("get build inputs: %w", fmt.Errorf("adopt inputs and pipes: %w", disaster))))
								Expect(needsReschedule).To(BeFalse())
							})
						})

						Context("when there are no next build inputs", func() {
							BeforeEach(func() {
								pendingBuild1.AdoptInputsAndPipesReturns(nil, false, nil)
							})

							It("doesn't return an error", func() {
								Expect(tryStartErr).NotTo(HaveOccurred())
								Expect(needsReschedule).To(BeFalse())
							})

							It("does not start the build", func() {
								Expect(createdBuild.StartCallCount()).To(BeZero())
							})
						})

						Context("when fetching pending builds fail", func() {
							BeforeEach(func() {
								job.GetPendingBuildsReturns(nil, disaster)
							})

							It("returns the error", func() {
								Expect(tryStartErr).To(Equal(fmt.Errorf("get pending builds: %w", disaster)))
							})

							It("does not need to be rescheduled", func() {
								Expect(needsReschedule).To(BeFalse())
							})
						})
					})

					Context("when there are several pending builds with one failing to start rerun build", func() {
						BeforeEach(func() {
							pendingBuild1 = new(dbfakes.FakeBuild)
							pendingBuild1.IDReturns(99)
							pendingBuild1.AdoptInputsAndPipesReturns([]db.BuildInput{{Name: "some-input"}}, true, nil)
							pendingBuild1.StartReturns(true, nil)
							job.ScheduleBuildReturnsOnCall(0, true, nil)
							pendingBuild2 = new(dbfakes.FakeBuild)
							pendingBuild2.IDReturns(999)
							pendingBuild2.AdoptInputsAndPipesReturns([]db.BuildInput{{Name: "some-input"}}, true, nil)
							pendingBuild2.StartReturns(true, nil)
							job.ScheduleBuildReturnsOnCall(2, true, nil)
						})

						Context("when the rerun build is failing to adopt inputs and outputs", func() {
							BeforeEach(func() {
								rerunBuild = new(dbfakes.FakeBuild)
								rerunBuild.IDReturns(555)
								rerunBuild.RerunOfReturns(pendingBuild1.ID())
								rerunBuild.AdoptRerunInputsAndPipesReturns(nil, false, errors.New("error"))
								job.ScheduleBuildReturnsOnCall(1, true, nil)
								pendingBuilds = []db.Build{pendingBuild1, rerunBuild, pendingBuild2}
								job.GetPendingBuildsReturns(pendingBuilds, nil)
							})

							It("does not schedule the next build", func() {
								Expect(tryStartErr).To(HaveOccurred())
								Expect(pendingBuild1.StartCallCount()).To(Equal(1))
								Expect(pendingBuild2.StartCallCount()).To(Equal(0))
							})
						})

						Context("when the rerun build is not started because it has no inputs or versions", func() {
							BeforeEach(func() {
								rerunBuild = new(dbfakes.FakeBuild)
								rerunBuild.IDReturns(555)
								rerunBuild.RerunOfReturns(pendingBuild1.ID())
								rerunBuild.AdoptRerunInputsAndPipesReturns(nil, false, nil)
								job.ScheduleBuildReturnsOnCall(1, true, nil)
								pendingBuilds = []db.Build{pendingBuild1, rerunBuild, pendingBuild2}
								job.GetPendingBuildsReturns(pendingBuilds, nil)
							})

							It("tries to schedule the 2 other pending builds", func() {
								Expect(tryStartErr).ToNot(HaveOccurred())
								Expect(needsReschedule).To(BeFalse())
								Expect(pendingBuild1.StartCallCount()).To(Equal(1))
								Expect(pendingBuild2.StartCallCount()).To(Equal(1))
							})
						})

						Context("when the rerun build needs to retry a new scheduler tick", func() {
							BeforeEach(func() {
								rerunBuild = new(dbfakes.FakeBuild)
								rerunBuild.IDReturns(555)
								rerunBuild.RerunOfReturns(pendingBuild1.ID())
								job.ScheduleBuildReturnsOnCall(1, false, nil)
								pendingBuilds = []db.Build{pendingBuild1, rerunBuild, pendingBuild2}
								job.GetPendingBuildsReturns(pendingBuilds, nil)
							})

							It("does not try to schedule the other pending build", func() {
								Expect(tryStartErr).ToNot(HaveOccurred())
								Expect(needsReschedule).To(BeTrue())
								Expect(pendingBuild1.StartCallCount()).To(Equal(1))
								Expect(pendingBuild2.StartCallCount()).To(Equal(0))
							})
						})
					})
				})
			})
		})
	})
})
