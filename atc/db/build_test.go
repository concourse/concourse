package db_test

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/event"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gocache "github.com/patrickmn/go-cache"
)

var _ = Describe("Build", func() {
	var (
		team       db.Team
		versionsDB db.VersionsDB

		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()

		var err error
		team, err = teamFactory.CreateTeam(atc.Team{Name: "some-team"})
		Expect(err).ToNot(HaveOccurred())

		versionsDB = db.NewVersionsDB(dbConn, 100, gocache.New(10*time.Second, 10*time.Second))
	})

	It("has no plan on creation", func() {
		var err error
		build, err := team.CreateOneOffBuild()
		Expect(err).ToNot(HaveOccurred())
		Expect(build.HasPlan()).To(BeFalse())
	})

	Describe("Reload", func() {
		It("updates the model", func() {
			build, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			started, err := build.Start(atc.Plan{})
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			Expect(build.Status()).To(Equal(db.BuildStatusPending))

			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(db.BuildStatusStarted))
		})
	})

	Describe("Drain", func() {
		It("defaults drain to false in the beginning", func() {
			build, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			Expect(build.IsDrained()).To(BeFalse())
		})

		It("has drain set to true after a drain and a reload", func() {
			build, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			err = build.SetDrained(true)
			Expect(err).NotTo(HaveOccurred())

			drained := build.IsDrained()
			Expect(drained).To(BeTrue())

			_, err = build.Reload()
			Expect(err).NotTo(HaveOccurred())
			drained = build.IsDrained()
			Expect(drained).To(BeTrue())
		})
	})

	Describe("Start", func() {
		var err error
		var started bool
		var build db.Build
		var plan atc.Plan

		BeforeEach(func() {
			plan = atc.Plan{
				ID: atc.PlanID("56"),
				Get: &atc.GetPlan{
					Type:     "some-type",
					Name:     "some-name",
					Resource: "some-resource",
					Source:   atc.Source{"some": "source"},
					Params:   atc.Params{"some": "params"},
					Version:  &atc.Version{"some": "version"},
					Tags:     atc.Tags{"some-tags"},
					VersionedResourceTypes: atc.VersionedResourceTypes{
						{
							ResourceType: atc.ResourceType{
								Name:       "some-name",
								Source:     atc.Source{"some": "source"},
								Type:       "some-type",
								Privileged: true,
								Tags:       atc.Tags{"some-tags"},
							},
							Version: atc.Version{"some-resource-type": "version"},
						},
					},
				},
			}

			build, err = team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			started, err = build.Start(plan)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("build has been aborted", func() {
			BeforeEach(func() {
				err = build.MarkAsAborted()
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not start the build", func() {
				Expect(started).To(BeFalse())
			})

			It("leaves the build in pending state", func() {
				found, err := build.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.Status()).To(Equal(db.BuildStatusPending))
			})
		})

		Context("build has not been aborted", func() {
			It("starts the build", func() {
				Expect(started).To(BeTrue())
			})

			It("creates Start event", func() {
				found, err := build.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.Status()).To(Equal(db.BuildStatusStarted))

				events, err := build.Events(0)
				Expect(err).NotTo(HaveOccurred())

				defer db.Close(events)

				Expect(events.Next()).To(Equal(envelope(event.Status{
					Status: atc.StatusStarted,
					Time:   build.StartTime().Unix(),
				})))
			})

			It("updates build status", func() {
				found, err := build.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.Status()).To(Equal(db.BuildStatusStarted))
			})

			It("saves the public plan", func() {
				found, err := build.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.HasPlan()).To(BeTrue())
				Expect(build.PublicPlan()).To(Equal(plan.Public()))
			})
		})
	})

	Describe("Finish", func() {
		var pipeline db.Pipeline
		var build db.Build
		var expectedOutputs []db.AlgorithmVersion
		var job db.Job

		BeforeEach(func() {
			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "some-type",
			}

			_, err = brt.FindOrCreate(setupTx, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name: "some-resource",
								},
							},
						},
					},
					{
						Name: "downstream-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name:   "some-resource",
									Passed: []string{"some-job"},
								},
							},
						},
					},
					{
						Name: "no-request-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name:   "some-resource",
									Passed: []string{"downstream-job"},
								},
							},
						},
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					},
					{
						Name:   "some-other-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "other-source"},
					},
				},
			}

			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())

			var found bool
			job, found, err = pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resource1, found, err := pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resourceConfigScope1, err := resource1.SetResourceConfig(atc.Source{"some": "source"}, atc.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			err = resourceConfigScope1.SaveVersions(nil, []atc.Version{
				{"ver": "1"},
				{"ver": "2"},
			})
			Expect(err).ToNot(HaveOccurred())

			resource2, found, err := pipeline.Resource("some-other-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resourceConfigScope2, err := resource2.SetResourceConfig(atc.Source{"some": "other-source"}, atc.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			err = resourceConfigScope2.SaveVersions(nil, []atc.Version{
				{"ver": "1"},
				{"ver": "2"},
				{"ver": "3"},
			})
			Expect(err).ToNot(HaveOccurred())

			build, err = job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			err = job.SaveNextInputMapping(db.InputMapping{
				"input-1": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "1"})),
							ResourceID: resource1.ID(),
						},
						FirstOccurrence: true,
					},
					PassedBuildIDs: []int{},
				},
				"input-2": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "3"})),
							ResourceID: resource2.ID(),
						},
						FirstOccurrence: true,
					},
					PassedBuildIDs: []int{},
				},
				"input-3": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "2"})),
							ResourceID: resource1.ID(),
						},
						FirstOccurrence: true,
					},
					PassedBuildIDs: []int{},
				},
				"input-4": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "2"})),
							ResourceID: resource1.ID(),
						},
						FirstOccurrence: true,
					},
					PassedBuildIDs: []int{},
				},
			}, true)
			Expect(err).NotTo(HaveOccurred())

			_, found, err = build.AdoptInputsAndPipes()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			// save explicit output from 'put'
			err = build.SaveOutput("some-type", atc.Source{"some": "source"}, atc.VersionedResourceTypes{}, atc.Version{"ver": "2"}, nil, "output-1", "some-resource")
			Expect(err).NotTo(HaveOccurred())

			// save explicit output from 'put'
			err = build.SaveOutput("some-type", atc.Source{"some": "source"}, atc.VersionedResourceTypes{}, atc.Version{"ver": "3"}, nil, "output-2", "some-resource")
			Expect(err).NotTo(HaveOccurred())

			err = build.Finish(db.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			expectedOutputs = []db.AlgorithmVersion{
				{
					Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "1"})),
					ResourceID: resource1.ID(),
				},
				{
					Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "3"})),
					ResourceID: resource2.ID(),
				},
				{
					Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "2"})),
					ResourceID: resource1.ID(),
				},
				{
					Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "3"})),
					ResourceID: resource1.ID(),
				},
			}
		})

		It("creates Finish event", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(db.BuildStatusSucceeded))

			events, err := build.Events(0)
			Expect(err).NotTo(HaveOccurred())

			defer db.Close(events)

			Expect(events.Next()).To(Equal(envelope(event.Status{
				Status: atc.StatusSucceeded,
				Time:   build.EndTime().Unix(),
			})))
		})

		It("updates build status", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(db.BuildStatusSucceeded))
		})

		Context("rerunning a build", func() {
			var (
				pdBuild, pdBuild2, rrBuild db.Build
				err                        error
				latestCompletedBuildCol    = "latest_completed_build_id"
				nextBuildCol               = "next_build_id"
				transitionBuildCol         = "transition_build_id"
			)

			Context("when there is a pending build that is not a rerun", func() {
				BeforeEach(func() {
					pdBuild, err = job.CreateBuild()
					Expect(err).NotTo(HaveOccurred())
				})

				Context("when rerunning the latest completed build", func() {
					BeforeEach(func() {
						rrBuild, err = job.RerunBuild(build)
						Expect(err).NotTo(HaveOccurred())
					})

					Context("when the rerun finishes and status changed", func() {
						BeforeEach(func() {
							err = rrBuild.Finish(db.BuildStatusFailed)
							Expect(err).NotTo(HaveOccurred())
						})

						It("updates job latest finished build id", func() {
							Expect(getJobBuildID(latestCompletedBuildCol, job.ID())).To(Equal(rrBuild.ID()))
						})

						It("updates job next build id to the pending build", func() {
							Expect(getJobBuildID(nextBuildCol, job.ID())).To(Equal(pdBuild.ID()))
						})

						It("updates transition build id to the rerun build", func() {
							Expect(getJobBuildID(transitionBuildCol, job.ID())).To(Equal(rrBuild.ID()))
						})
					})

					Context("when there is another pending build that is not a rerun and the first pending build finishes", func() {
						BeforeEach(func() {
							pdBuild2, err = job.CreateBuild()
							Expect(err).NotTo(HaveOccurred())

							err = pdBuild.Finish(db.BuildStatusSucceeded)
							Expect(err).NotTo(HaveOccurred())
						})

						It("updates job next build id to be the next non rerun pending build", func() {
							Expect(getJobBuildID(nextBuildCol, job.ID())).To(Equal(pdBuild2.ID()))
						})

						It("updates job latest finished build id", func() {
							Expect(getJobBuildID(latestCompletedBuildCol, job.ID())).To(Equal(pdBuild.ID()))
						})
					})
				})

				Context("when rerunning the pending build and the pending build finished", func() {
					BeforeEach(func() {
						rrBuild, err = job.RerunBuild(pdBuild)
						Expect(err).NotTo(HaveOccurred())

						err = pdBuild.Finish(db.BuildStatusSucceeded)
						Expect(err).NotTo(HaveOccurred())
					})

					It("updates job next build id to the rerun build", func() {
						Expect(getJobBuildID(nextBuildCol, job.ID())).To(Equal(rrBuild.ID()))
					})

					It("updates job latest finished build id", func() {
						Expect(getJobBuildID(latestCompletedBuildCol, job.ID())).To(Equal(pdBuild.ID()))
					})

					Context("when rerunning the rerun build", func() {
						var rrBuild2 db.Build

						BeforeEach(func() {
							err = rrBuild.Finish(db.BuildStatusSucceeded)
							Expect(err).NotTo(HaveOccurred())

							rrBuild2, err = job.RerunBuild(rrBuild)
							Expect(err).NotTo(HaveOccurred())
						})

						It("updates job next build id to the rerun build", func() {
							Expect(getJobBuildID(nextBuildCol, job.ID())).To(Equal(rrBuild2.ID()))
						})

						It("updates job latest finished build id", func() {
							Expect(getJobBuildID(latestCompletedBuildCol, job.ID())).To(Equal(rrBuild.ID()))
						})
					})
				})

				Context("when pending build finished and rerunning a non latest build and it finishes", func() {
					BeforeEach(func() {
						err = pdBuild.Finish(db.BuildStatusErrored)
						Expect(err).NotTo(HaveOccurred())

						rrBuild, err = job.RerunBuild(build)
						Expect(err).NotTo(HaveOccurred())

						err = rrBuild.Finish(db.BuildStatusSucceeded)
						Expect(err).NotTo(HaveOccurred())
					})

					It("updates job next build id to nul", func() {
						_, nextBuild, err := job.FinishedAndNextBuild()
						Expect(err).NotTo(HaveOccurred())
						Expect(nextBuild).To(BeNil())
					})

					It("does not updates job latest finished build id", func() {
						Expect(getJobBuildID(latestCompletedBuildCol, job.ID())).To(Equal(pdBuild.ID()))
					})

					It("does not updates transition build id", func() {
						Expect(getJobBuildID(transitionBuildCol, job.ID())).To(Equal(pdBuild.ID()))
					})
				})
			})
		})

		It("clears out the private plan", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.PrivatePlan()).To(Equal(atc.Plan{}))
		})

		It("sets completed to true", func() {
			Expect(build.IsCompleted()).To(BeFalse())
			Expect(build.IsRunning()).To(BeTrue())

			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.IsCompleted()).To(BeTrue())
			Expect(build.IsRunning()).To(BeFalse())
		})

		It("inserts inputs and outputs into successful build versions", func() {
			outputs, err := versionsDB.SuccessfulBuildOutputs(ctx, build.ID())
			Expect(err).NotTo(HaveOccurred())
			Expect(outputs).To(ConsistOf(expectedOutputs))
		})

		Context("when requesting schedule", func() {
			It("request schedule on the downstream job", func() {
				downstreamJob, found, err := pipeline.Job("downstream-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				newBuild, err := job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				requestedSchedule := downstreamJob.ScheduleRequestedTime()

				err = newBuild.Finish(db.BuildStatusSucceeded)
				Expect(err).NotTo(HaveOccurred())

				found, err = downstreamJob.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(downstreamJob.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule))
			})
			It("do not request schedule on jobs that are not directly downstream", func() {
				noRequestJob, found, err := pipeline.Job("no-request-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				newBuild, err := job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				requestedSchedule := noRequestJob.ScheduleRequestedTime()

				err = newBuild.Finish(db.BuildStatusSucceeded)
				Expect(err).NotTo(HaveOccurred())

				found, err = noRequestJob.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(noRequestJob.ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule))
			})
		})

		Context("archiving pipelines", func() {
			var childPipeline db.Pipeline

			BeforeEach(func() {
				By("creating a child pipeline")
				build, _ := defaultJob.CreateBuild()
				childPipeline, _, _ = build.SavePipeline("child1-pipeline", defaultTeam.ID(), defaultPipelineConfig, db.ConfigVersion(0), false)
				build.Finish(db.BuildStatusSucceeded)

				childPipeline.Reload()
				Expect(childPipeline.Archived()).To(BeFalse())
			})

			Context("build is successful", func() {
				It("archives pipelines no longer set by the job", func() {
					By("no longer setting the child pipeline")
					build2, _ := defaultJob.CreateBuild()
					build2.Finish(db.BuildStatusSucceeded)

					childPipeline.Reload()
					Expect(childPipeline.Archived()).To(BeTrue())
				})

				Context("chain of pipelines setting each other... like a russian doll set...", func() {
					It("archives all descendent pipelines", func() {
						childPipelines := []db.Pipeline{childPipeline}

						By("creating a chain of pipelines, previous pipeline setting the next pipeline")
						for i := 0; i < 5; i++ {
							job, _, _ := childPipeline.Job("some-job")
							build, _ := job.CreateBuild()
							childPipeline, _, _ = build.SavePipeline("child-pipeline-"+strconv.Itoa(i), defaultTeam.ID(), defaultPipelineConfig, db.ConfigVersion(0), false)
							build.Finish(db.BuildStatusSucceeded)
							childPipelines = append(childPipelines, childPipeline)
						}

						By("parent pipeline no longer sets child pipeline in most recent build")
						build, _ := defaultJob.CreateBuild()
						build.Finish(db.BuildStatusSucceeded)

						for _, pipeline := range childPipelines {
							pipeline.Reload()
							Expect(pipeline.Archived()).To(BeTrue())
						}

					})
				})

				Context("when the pipeline is not set by build", func() {
					It("never gets archived", func() {
						build, _ := defaultJob.CreateBuild()
						teamPipeline, _, _ := defaultTeam.SavePipeline("team-pipeline", defaultPipelineConfig, db.ConfigVersion(0), false)
						build.Finish(db.BuildStatusSucceeded)

						teamPipeline.Reload()
						Expect(teamPipeline.Archived()).To(BeFalse())
					})
				})
			})
			Context("build is not successful", func() {
				It("does not archive pipelines", func() {
					By("no longer setting the child pipeline")
					build2, _ := defaultJob.CreateBuild()
					build2.Finish(db.BuildStatusFailed)

					childPipeline.Reload()
					Expect(childPipeline.Archived()).To(BeFalse())
				})
			})
		})
	})

	Describe("Abort", func() {
		var build db.Build
		BeforeEach(func() {
			var err error
			build, err = team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			err = build.MarkAsAborted()
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates aborted to true", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.IsAborted()).To(BeTrue())
		})
	})

	Describe("Events", func() {
		It("saves and emits status events", func() {
			build, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			By("allowing you to subscribe when no events have yet occurred")
			events, err := build.Events(0)
			Expect(err).NotTo(HaveOccurred())

			defer db.Close(events)

			By("emitting a status event when started")
			started, err := build.Start(atc.Plan{})
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(events.Next()).To(Equal(envelope(event.Status{
				Status: atc.StatusStarted,
				Time:   build.StartTime().Unix(),
			})))

			By("emitting a status event when finished")
			err = build.Finish(db.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			found, err = build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(events.Next()).To(Equal(envelope(event.Status{
				Status: atc.StatusSucceeded,
				Time:   build.EndTime().Unix(),
			})))

			By("ending the stream when finished")
			_, err = events.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))
		})
	})

	Describe("SaveEvent", func() {
		It("saves and propagates events correctly", func() {
			build, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			By("allowing you to subscribe when no events have yet occurred")
			events, err := build.Events(0)
			Expect(err).NotTo(HaveOccurred())

			defer db.Close(events)

			By("saving them in order")
			err = build.SaveEvent(event.Log{
				Payload: "some ",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(events.Next()).To(Equal(envelope(event.Log{
				Payload: "some ",
			})))

			err = build.SaveEvent(event.Log{
				Payload: "log",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(events.Next()).To(Equal(envelope(event.Log{
				Payload: "log",
			})))

			By("allowing you to subscribe from an offset")
			eventsFrom1, err := build.Events(1)
			Expect(err).NotTo(HaveOccurred())

			defer db.Close(eventsFrom1)

			Expect(eventsFrom1.Next()).To(Equal(envelope(event.Log{
				Payload: "log",
			})))

			By("notifying those waiting on events as soon as they're saved")
			nextEvent := make(chan event.Envelope)
			nextErr := make(chan error)

			go func() {
				event, err := events.Next()
				if err != nil {
					nextErr <- err
				} else {
					nextEvent <- event
				}
			}()

			Consistently(nextEvent).ShouldNot(Receive())
			Consistently(nextErr).ShouldNot(Receive())

			err = build.SaveEvent(event.Log{
				Payload: "log 2",
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(nextEvent).Should(Receive(Equal(envelope(event.Log{
				Payload: "log 2",
			}))))

			By("returning ErrBuildEventStreamClosed for Next calls after Close")
			events3, err := build.Events(0)
			Expect(err).NotTo(HaveOccurred())

			err = events3.Close()
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				_, err := events3.Next()
				return err
			}).Should(Equal(db.ErrBuildEventStreamClosed))
		})
	})

	Describe("SaveOutput", func() {
		var pipeline db.Pipeline
		var job db.Job
		var resourceConfigScope db.ResourceConfigScope

		BeforeEach(func() {
			atc.EnableGlobalResources = true

			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name: "some-implicit-resource",
						Type: "some-type",
					},
					{
						Name:   "some-explicit-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "explicit-source"},
					},
				},
			}

			var err error
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())

			var found bool
			job, found, err = pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "some-type",
			}

			_, err = brt.FindOrCreate(setupTx, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			resource, found, err := pipeline.Resource("some-explicit-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resourceConfigScope, err = resource.SetResourceConfig(atc.Source{"some": "explicit-source"}, atc.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the version does not exist", func() {
			It("can save a build's output", func() {
				build, err := job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				err = build.SaveOutput("some-type", atc.Source{"some": "explicit-source"}, atc.VersionedResourceTypes{}, atc.Version{"some": "version"}, []db.ResourceConfigMetadataField{
					{
						Name:  "meta1",
						Value: "data1",
					},
					{
						Name:  "meta2",
						Value: "data2",
					},
				}, "output-name", "some-explicit-resource")
				Expect(err).ToNot(HaveOccurred())

				rcv, found, err := resourceConfigScope.FindVersion(atc.Version{"some": "version"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				_, buildOutputs, err := build.Resources()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(buildOutputs)).To(Equal(1))
				Expect(buildOutputs[0].Name).To(Equal("output-name"))
				Expect(buildOutputs[0].Version).To(Equal(atc.Version(rcv.Version())))
			})

			It("requests schedule on all jobs using the resource config", func() {
				atc.EnableGlobalResources = true

				build, err := job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				pipelineConfig := atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
							PlanSequence: []atc.Step{
								{
									Config: &atc.GetStep{
										Name: "some-explicit-resource",
									},
								},
							},
						},
						{
							Name: "other-job",
						},
					},
					Resources: atc.ResourceConfigs{
						{
							Name:   "some-explicit-resource",
							Type:   "some-type",
							Source: atc.Source{"some": "explicit-source"},
						},
					},
				}

				otherPipeline, _, err := team.SavePipeline("some-other-pipeline", pipelineConfig, db.ConfigVersion(1), false)
				Expect(err).ToNot(HaveOccurred())

				resource, found, err := otherPipeline.Resource("some-explicit-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				resourceConfigScope, err = resource.SetResourceConfig(atc.Source{"some": "explicit-source"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				requestedJob, found, err := otherPipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				otherJob, found, err := otherPipeline.Job("other-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				requestedSchedule1 := requestedJob.ScheduleRequestedTime()
				requestedSchedule2 := otherJob.ScheduleRequestedTime()

				err = build.SaveOutput("some-type", atc.Source{"some": "explicit-source"}, atc.VersionedResourceTypes{}, atc.Version{"some": "version"}, []db.ResourceConfigMetadataField{}, "output-name", "some-explicit-resource")
				Expect(err).ToNot(HaveOccurred())

				found, err = requestedJob.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				found, err = otherJob.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(requestedJob.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule1))
				Expect(otherJob.ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule2))
			})
		})

		Context("when the version already exists", func() {
			var rcv db.ResourceConfigVersion

			BeforeEach(func() {
				err := resourceConfigScope.SaveVersions(nil, []atc.Version{{"some": "version"}})
				Expect(err).ToNot(HaveOccurred())

				var found bool
				rcv, found, err = resourceConfigScope.FindVersion(atc.Version{"some": "version"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("does not increment the check order", func() {
				build, err := job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				err = build.SaveOutput("some-type", atc.Source{"some": "explicit-source"}, atc.VersionedResourceTypes{}, atc.Version{"some": "version"}, []db.ResourceConfigMetadataField{
					{
						Name:  "meta1",
						Value: "data1",
					},
					{
						Name:  "meta2",
						Value: "data2",
					},
				}, "output-name", "some-explicit-resource")
				Expect(err).ToNot(HaveOccurred())

				newRCV, found, err := resourceConfigScope.FindVersion(atc.Version{"some": "version"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(newRCV.CheckOrder()).To(Equal(rcv.CheckOrder()))
			})

			It("does not request schedule on all jobs using the resource config", func() {
				build, err := job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				pipelineConfig := atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
							PlanSequence: []atc.Step{
								{
									Config: &atc.GetStep{
										Name: "some-explicit-resource",
									},
								},
							},
						},
						{
							Name: "other-job",
						},
					},
					Resources: atc.ResourceConfigs{
						{
							Name:   "some-explicit-resource",
							Type:   "some-type",
							Source: atc.Source{"some": "explicit-source"},
						},
					},
				}

				otherPipeline, _, err := team.SavePipeline("some-other-pipeline", pipelineConfig, db.ConfigVersion(1), false)
				Expect(err).ToNot(HaveOccurred())

				resource, found, err := otherPipeline.Resource("some-explicit-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				resourceConfigScope, err = resource.SetResourceConfig(atc.Source{"some": "explicit-source"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				requestedJob, found, err := otherPipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				requestedSchedule1 := requestedJob.ScheduleRequestedTime()

				err = build.SaveOutput("some-type", atc.Source{"some": "explicit-source"}, atc.VersionedResourceTypes{}, atc.Version{"some": "version"}, []db.ResourceConfigMetadataField{}, "output-name", "some-explicit-resource")
				Expect(err).ToNot(HaveOccurred())

				found, err = requestedJob.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(requestedJob.ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule1))
			})
		})

		Context("when global resources is enabled", func() {
			BeforeEach(func() {
				atc.EnableGlobalResources = true
			})

			Context("using a given resource type", func() {
				var givenType string

				BeforeEach(func() {
					givenType = "given-type"
				})

				Context("the resource types contain the given type", func() {
					var resourceTypes atc.VersionedResourceTypes

					BeforeEach(func() {
						resourceTypes = atc.VersionedResourceTypes{
							{
								ResourceType: atc.ResourceType{
									Name:                 "given-type",
									Source:               atc.Source{"some": "source"},
									Type:                 "some-type",
									UniqueVersionHistory: true,
								},
								Version: atc.Version{"some-resource-type": "version"},
							},
						}
					})

					Context("but the resource type is different in the db", func() {
						var resourceName string

						BeforeEach(func() {
							resourceName = "some-explicit-resource" // type: "some-type"
						})

						It("saves the output", func() {
							build, err := job.CreateBuild()
							Expect(err).ToNot(HaveOccurred())

							err = build.SaveOutput(
								givenType,
								atc.Source{"some": "explicit-source"},
								resourceTypes,
								atc.Version{"some": "new-version"},
								[]db.ResourceConfigMetadataField{},
								"output-name",
								resourceName,
							)
							Expect(err).ToNot(HaveOccurred())
						})
					})
				})
			})
		})
	})

	Describe("Resources", func() {
		var (
			pipeline             db.Pipeline
			job                  db.Job
			resourceConfigScope1 db.ResourceConfigScope
			resource1            db.Resource
			found                bool
		)

		BeforeEach(func() {
			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "some-type",
			}

			_, err = brt.FindOrCreate(setupTx, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					},
					{
						Name:   "some-other-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source-2"},
					},
					{
						Name:   "some-unused-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source-3"},
					},
				},
			}

			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())

			job, found, err = pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resource1, found, err = pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resource2, found, err := pipeline.Resource("some-other-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resourceConfigScope1, err = resource1.SetResourceConfig(atc.Source{"some": "source-1"}, atc.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			_, err = resource2.SetResourceConfig(atc.Source{"some": "source-2"}, atc.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			err = resourceConfigScope1.SaveVersions(nil, []atc.Version{
				{"ver": "1"},
				{"ver": "2"},
			})
			Expect(err).ToNot(HaveOccurred())

			// This version should not be returned by the Resources method because it has a check order of 0
			created, err := resource1.SaveUncheckedVersion(atc.Version{"ver": "not-returned"}, nil, resourceConfigScope1.ResourceConfig(), atc.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())
			Expect(created).To(BeTrue())
		})

		It("returns build inputs and outputs", func() {
			build, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			// save a normal 'get'
			err = job.SaveNextInputMapping(db.InputMapping{
				"some-input": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "1"})),
							ResourceID: resource1.ID(),
						},
						FirstOccurrence: true,
					},
					PassedBuildIDs: []int{},
				},
			}, true)
			Expect(err).NotTo(HaveOccurred())

			_, found, err := build.AdoptInputsAndPipes()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			// save explicit output from 'put'
			err = build.SaveOutput("some-type", atc.Source{"some": "source-2"}, atc.VersionedResourceTypes{}, atc.Version{"ver": "2"}, nil, "some-output-name", "some-other-resource")
			Expect(err).NotTo(HaveOccurred())

			inputs, outputs, err := build.Resources()
			Expect(err).NotTo(HaveOccurred())

			Expect(inputs).To(ConsistOf([]db.BuildInput{
				{Name: "some-input", Version: atc.Version{"ver": "1"}, ResourceID: resource1.ID(), FirstOccurrence: true},
			}))

			Expect(outputs).To(ConsistOf([]db.BuildOutput{
				{
					Name:    "some-output-name",
					Version: atc.Version{"ver": "2"},
				},
			}))
		})

		It("can't get no satisfaction (resources from a one-off build)", func() {
			oneOffBuild, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			inputs, outputs, err := oneOffBuild.Resources()
			Expect(err).NotTo(HaveOccurred())

			Expect(inputs).To(BeEmpty())
			Expect(outputs).To(BeEmpty())
		})

		Context("when the first occurrence is empty", func() {
			var build db.Build

			BeforeEach(func() {
				var err error
				build, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				// save a normal 'get'
				err = job.SaveNextInputMapping(db.InputMapping{
					"some-input": db.InputResult{
						Input: &db.AlgorithmInput{
							AlgorithmVersion: db.AlgorithmVersion{
								Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "1"})),
								ResourceID: resource1.ID(),
							},
							FirstOccurrence: true,
						},
						PassedBuildIDs: []int{},
					},
					"some-other-input": db.InputResult{
						Input: &db.AlgorithmInput{
							AlgorithmVersion: db.AlgorithmVersion{
								Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "2"})),
								ResourceID: resource1.ID(),
							},
							FirstOccurrence: true,
						},
						PassedBuildIDs: []int{},
					},
				}, true)
				Expect(err).NotTo(HaveOccurred())

				_, found, err := build.AdoptInputsAndPipes()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				_, err = psql.Update("build_resource_config_version_inputs").
					Set("first_occurrence", nil).
					Where(sq.Eq{
						"build_id":    build.ID(),
						"resource_id": resource1.ID(),
						"version_md5": convertToMD5(atc.Version{"ver": "1"}),
					}).
					RunWith(dbConn).
					Exec()
				Expect(err).NotTo(HaveOccurred())
			})

			It("determines the first occurrence to be true", func() {
				inputs, outputs, err := build.Resources()
				Expect(err).NotTo(HaveOccurred())
				Expect(outputs).To(BeEmpty())
				Expect(inputs).To(ConsistOf([]db.BuildInput{
					{Name: "some-input", Version: atc.Version{"ver": "1"}, ResourceID: resource1.ID(), FirstOccurrence: true},
					{Name: "some-other-input", Version: atc.Version{"ver": "2"}, ResourceID: resource1.ID(), FirstOccurrence: true},
				}))
			})

			Context("when the a build with those inputs already exist", func() {
				var newBuild db.Build

				BeforeEach(func() {
					var err error
					newBuild, err = job.CreateBuild()
					Expect(err).NotTo(HaveOccurred())

					// save a normal 'get'
					err = job.SaveNextInputMapping(db.InputMapping{
						"some-input": db.InputResult{
							Input: &db.AlgorithmInput{
								AlgorithmVersion: db.AlgorithmVersion{
									Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "1"})),
									ResourceID: resource1.ID(),
								},
								FirstOccurrence: false,
							},
							PassedBuildIDs: []int{},
						},
					}, true)
					Expect(err).NotTo(HaveOccurred())

					_, found, err := newBuild.AdoptInputsAndPipes()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					_, err = psql.Update("build_resource_config_version_inputs").
						Set("first_occurrence", nil).
						Where(sq.Eq{
							"build_id":    newBuild.ID(),
							"resource_id": resource1.ID(),
							"version_md5": convertToMD5(atc.Version{"ver": "1"}),
						}).
						RunWith(dbConn).
						Exec()
					Expect(err).NotTo(HaveOccurred())
				})

				It("determines the first occurrence to be false", func() {
					inputs, outputs, err := newBuild.Resources()
					Expect(err).NotTo(HaveOccurred())
					Expect(outputs).To(BeEmpty())
					Expect(inputs).To(ConsistOf([]db.BuildInput{
						{Name: "some-input", Version: atc.Version{"ver": "1"}, ResourceID: resource1.ID(), FirstOccurrence: false},
					}))
				})
			})
		})
	})

	Describe("Pipeline", func() {
		var (
			build           db.Build
			foundPipeline   db.Pipeline
			createdPipeline db.Pipeline
			found           bool
		)

		JustBeforeEach(func() {
			var err error
			foundPipeline, found, err = build.Pipeline()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when a job build", func() {
			BeforeEach(func() {
				var err error
				createdPipeline, _, err = team.SavePipeline("some-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
						},
					},
				}, db.ConfigVersion(1), false)
				Expect(err).ToNot(HaveOccurred())

				job, found, err := createdPipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err = job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the correct pipeline", func() {
				Expect(found).To(BeTrue())
				Expect(foundPipeline.Name()).To(Equal(createdPipeline.Name()))
			})
		})

		Context("when a one off build", func() {
			BeforeEach(func() {
				var err error
				build, err = team.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not return a pipeline", func() {
				Expect(found).To(BeFalse())
				Expect(foundPipeline).To(BeNil())
			})
		})
	})

	Describe("Preparation", func() {
		var (
			build             db.Build
			err               error
			expectedBuildPrep db.BuildPreparation
		)
		BeforeEach(func() {
			expectedBuildPrep = db.BuildPreparation{
				BuildID:             123456789,
				PausedPipeline:      db.BuildPreparationStatusNotBlocking,
				PausedJob:           db.BuildPreparationStatusNotBlocking,
				MaxRunningBuilds:    db.BuildPreparationStatusNotBlocking,
				Inputs:              map[string]db.BuildPreparationStatus{},
				InputsSatisfied:     db.BuildPreparationStatusNotBlocking,
				MissingInputReasons: db.MissingInputReasons{},
			}
		})

		Context("for one-off build", func() {
			BeforeEach(func() {
				build, err = team.CreateOneOffBuild()
				Expect(err).NotTo(HaveOccurred())

				expectedBuildPrep.BuildID = build.ID()
			})

			It("returns build preparation", func() {
				buildPrep, found, err := build.Preparation()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(buildPrep).To(Equal(expectedBuildPrep))
			})

			Context("when the build is started", func() {
				BeforeEach(func() {
					started, err := build.Start(atc.Plan{})
					Expect(started).To(BeTrue())
					Expect(err).NotTo(HaveOccurred())

					stillExists, err := build.Reload()
					Expect(stillExists).To(BeTrue())
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns build preparation", func() {
					buildPrep, found, err := build.Preparation()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(buildPrep).To(Equal(expectedBuildPrep))
				})
			})
		})

		Context("for job build", func() {
			var (
				pipeline db.Pipeline
				job      db.Job
			)

			BeforeEach(func() {
				var err error
				pipeline, _, err = team.SavePipeline("some-pipeline", atc.Config{
					Resources: atc.ResourceConfigs{
						{
							Name: "some-resource",
							Type: "some-type",
							Source: atc.Source{
								"source-config": "some-value",
							},
						},
					},
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
							PlanSequence: []atc.Step{
								{
									Config: &atc.GetStep{
										Name:     "some-input",
										Resource: "some-resource",
									},
								},
							},
						},
					},
				}, db.ConfigVersion(1), false)
				Expect(err).ToNot(HaveOccurred())

				var found bool
				job, found, err = pipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				expectedBuildPrep.BuildID = build.ID()

				job, found, err = pipeline.Job("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			Context("when inputs are satisfied", func() {
				var (
					resourceConfigScope db.ResourceConfigScope
					resource            db.Resource
					rcv                 db.ResourceConfigVersion
					err                 error
				)

				BeforeEach(func() {
					setupTx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())

					brt := db.BaseResourceType{
						Name: "some-type",
					}

					_, err = brt.FindOrCreate(setupTx, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(setupTx.Commit()).To(Succeed())

					var found bool
					resource, found, err = pipeline.Resource("some-resource")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					resourceConfigScope, err = resource.SetResourceConfig(atc.Source{"some": "source"}, atc.VersionedResourceTypes{})
					Expect(err).NotTo(HaveOccurred())

					err = resourceConfigScope.SaveVersions(nil, []atc.Version{{"version": "v5"}})
					Expect(err).NotTo(HaveOccurred())

					rcv, found, err = resourceConfigScope.FindVersion(atc.Version{"version": "v5"})
					Expect(found).To(BeTrue())
					Expect(err).NotTo(HaveOccurred())

					err = job.SaveNextInputMapping(db.InputMapping{
						"some-input": db.InputResult{
							Input: &db.AlgorithmInput{
								AlgorithmVersion: db.AlgorithmVersion{
									Version:    db.ResourceVersion(convertToMD5(atc.Version(rcv.Version()))),
									ResourceID: resource.ID(),
								},
								FirstOccurrence: true,
							},
							PassedBuildIDs: []int{},
						},
					}, true)
					Expect(err).NotTo(HaveOccurred())

					expectedBuildPrep.Inputs = map[string]db.BuildPreparationStatus{
						"some-input": db.BuildPreparationStatusNotBlocking,
					}
				})

				Context("when resource check finished after build created", func() {
					BeforeEach(func() {
						updated, err := resourceConfigScope.UpdateLastCheckEndTime()
						Expect(err).NotTo(HaveOccurred())
						Expect(updated).To(BeTrue())

						reloaded, err := resource.Reload()
						Expect(err).NotTo(HaveOccurred())
						Expect(reloaded).To(BeTrue())

						lastCheckEndTime := resource.LastCheckEndTime()
						Expect(lastCheckEndTime.IsZero()).To(BeFalse())

						err = job.SaveNextInputMapping(db.InputMapping{
							"some-input": db.InputResult{
								Input: &db.AlgorithmInput{
									AlgorithmVersion: db.AlgorithmVersion{
										Version:    db.ResourceVersion(convertToMD5(atc.Version(rcv.Version()))),
										ResourceID: resource.ID(),
									},
									FirstOccurrence: true,
								},
							},
						}, true)
						Expect(err).NotTo(HaveOccurred())

						expectedBuildPrep.Inputs = map[string]db.BuildPreparationStatus{
							"some-input": db.BuildPreparationStatusNotBlocking,
						}
					})

					Context("when the build is started", func() {
						BeforeEach(func() {
							started, err := build.Start(atc.Plan{})
							Expect(started).To(BeTrue())
							Expect(err).NotTo(HaveOccurred())

							stillExists, err := build.Reload()
							Expect(stillExists).To(BeTrue())
							Expect(err).NotTo(HaveOccurred())

							expectedBuildPrep.Inputs = map[string]db.BuildPreparationStatus{}
						})

						It("returns build preparation", func() {
							buildPrep, found, err := build.Preparation()
							Expect(err).NotTo(HaveOccurred())
							Expect(found).To(BeTrue())
							Expect(buildPrep).To(Equal(expectedBuildPrep))
						})
					})

					Context("when pipeline is paused", func() {
						BeforeEach(func() {
							err := pipeline.Pause()
							Expect(err).NotTo(HaveOccurred())

							expectedBuildPrep.PausedPipeline = db.BuildPreparationStatusBlocking
						})

						It("returns build preparation with paused pipeline", func() {
							buildPrep, found, err := build.Preparation()
							Expect(err).NotTo(HaveOccurred())
							Expect(found).To(BeTrue())
							Expect(buildPrep).To(Equal(expectedBuildPrep))
						})
					})

					Context("when job is paused", func() {
						BeforeEach(func() {
							err := job.Pause()
							Expect(err).NotTo(HaveOccurred())

							expectedBuildPrep.PausedJob = db.BuildPreparationStatusBlocking
						})

						It("returns build preparation with paused pipeline", func() {
							buildPrep, found, err := build.Preparation()
							Expect(err).NotTo(HaveOccurred())
							Expect(found).To(BeTrue())
							Expect(buildPrep).To(Equal(expectedBuildPrep))
						})
					})

					Context("when max running builds is reached", func() {
						BeforeEach(func() {
							var found bool
							var err error
							job, found, err = pipeline.Job("some-job")
							Expect(err).ToNot(HaveOccurred())
							Expect(found).To(BeTrue())

							newBuild, err := job.CreateBuild()
							Expect(err).NotTo(HaveOccurred())

							err = job.SaveNextInputMapping(nil, true)
							Expect(err).NotTo(HaveOccurred())

							scheduled, err := job.ScheduleBuild(newBuild)
							Expect(err).ToNot(HaveOccurred())
							Expect(scheduled).To(BeTrue())

							pipeline, _, err = team.SavePipeline("some-pipeline", atc.Config{
								Resources: atc.ResourceConfigs{
									{
										Name: "some-resource",
										Type: "some-type",
										Source: atc.Source{
											"source-config": "some-value",
										},
									},
								},
								Jobs: atc.JobConfigs{
									{
										Name:           "some-job",
										RawMaxInFlight: 1,
										PlanSequence: []atc.Step{
											{
												Config: &atc.GetStep{
													Name:     "some-input",
													Resource: "some-resource",
												},
											},
										},
									},
								},
							}, db.ConfigVersion(2), false)
							Expect(err).ToNot(HaveOccurred())

							job, found, err = pipeline.Job("some-job")
							Expect(err).NotTo(HaveOccurred())
							Expect(found).To(BeTrue())

							err = job.SaveNextInputMapping(db.InputMapping{
								"some-input": db.InputResult{
									Input: &db.AlgorithmInput{
										AlgorithmVersion: db.AlgorithmVersion{
											Version:    db.ResourceVersion(convertToMD5(atc.Version(rcv.Version()))),
											ResourceID: resource.ID(),
										},
										FirstOccurrence: true,
									},
									PassedBuildIDs: []int{},
								},
							}, true)
							Expect(err).NotTo(HaveOccurred())

							scheduled, err = job.ScheduleBuild(build)
							Expect(err).ToNot(HaveOccurred())
							Expect(scheduled).To(BeFalse())

							expectedBuildPrep.MaxRunningBuilds = db.BuildPreparationStatusBlocking
						})

						It("returns build preparation with max in flight reached", func() {
							buildPrep, found, err := build.Preparation()
							Expect(err).NotTo(HaveOccurred())
							Expect(found).To(BeTrue())
							Expect(buildPrep).To(Equal(expectedBuildPrep))
						})
					})

					Context("when max running builds is de-reached", func() {
						BeforeEach(func() {
							var err error
							pipeline, _, err = team.SavePipeline("some-pipeline", atc.Config{
								Resources: atc.ResourceConfigs{
									{
										Name: "some-resource",
										Type: "some-type",
										Source: atc.Source{
											"source-config": "some-value",
										},
									},
								},
								Jobs: atc.JobConfigs{
									{
										Name:           "some-job",
										RawMaxInFlight: 1,
										PlanSequence: []atc.Step{
											{
												Config: &atc.GetStep{
													Name:     "some-input",
													Resource: "some-resource",
												},
											},
										},
									},
								},
							}, db.ConfigVersion(2), false)
							Expect(err).ToNot(HaveOccurred())

							var found bool
							job, found, err = pipeline.Job("some-job")
							Expect(err).ToNot(HaveOccurred())
							Expect(found).To(BeTrue())

							newBuild, err := job.CreateBuild()
							Expect(err).NotTo(HaveOccurred())

							scheduled, err := job.ScheduleBuild(build)
							Expect(err).ToNot(HaveOccurred())
							Expect(scheduled).To(BeTrue())

							job, found, err = pipeline.Job("some-job")
							Expect(err).NotTo(HaveOccurred())
							Expect(found).To(BeTrue())

							err = newBuild.Finish(db.BuildStatusSucceeded)
							Expect(err).NotTo(HaveOccurred())
						})

						It("returns build preparation with max in flight not reached", func() {
							buildPrep, found, err := build.Preparation()
							Expect(err).NotTo(HaveOccurred())
							Expect(found).To(BeTrue())
							Expect(buildPrep).To(Equal(expectedBuildPrep))
						})
					})
				})

				Context("when no resource check finished after build created", func() {
					BeforeEach(func() {
						err = job.SaveNextInputMapping(db.InputMapping{
							"some-input": db.InputResult{
								Input: &db.AlgorithmInput{
									AlgorithmVersion: db.AlgorithmVersion{
										Version:    db.ResourceVersion(convertToMD5(atc.Version(rcv.Version()))),
										ResourceID: resource.ID(),
									},
									FirstOccurrence: true,
								},
								PassedBuildIDs: []int{},
							},
						}, false)
						Expect(err).NotTo(HaveOccurred())

						expectedBuildPrep.Inputs = map[string]db.BuildPreparationStatus{
							"some-input": db.BuildPreparationStatusBlocking,
						}
						expectedBuildPrep.InputsSatisfied = db.BuildPreparationStatusBlocking
						expectedBuildPrep.MissingInputReasons = db.MissingInputReasons{
							"some-input": db.NoResourceCheckFinished,
						}
					})

					It("returns build preparation with missing input reason", func() {
						buildPrep, found, err := build.Preparation()
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
						Expect(buildPrep).To(Equal(expectedBuildPrep))
					})
				})
			})

			Context("when inputs are not satisfied", func() {
				BeforeEach(func() {
					expectedBuildPrep.InputsSatisfied = db.BuildPreparationStatusBlocking
					expectedBuildPrep.MissingInputReasons = map[string]string{"some-input": db.MissingBuildInput}
					expectedBuildPrep.Inputs = map[string]db.BuildPreparationStatus{"some-input": db.BuildPreparationStatusBlocking}
				})

				It("returns blocking inputs satisfied", func() {
					buildPrep, found, err := build.Preparation()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(buildPrep).To(Equal(expectedBuildPrep))
				})
			})

			Context("when some inputs are errored", func() {
				BeforeEach(func() {
					pipelineConfig := atc.Config{
						Jobs: atc.JobConfigs{
							{
								Name: "some-job",
								PlanSequence: []atc.Step{
									{
										Config: &atc.GetStep{
											Name:    "input1",
											Version: &atc.VersionConfig{Pinned: atc.Version{"version": "v1"}},
										},
									},
									{
										Config: &atc.GetStep{
											Name: "input2",
										},
									},
									{
										Config: &atc.GetStep{
											Name:   "input3",
											Passed: []string{"some-upstream-job"},
										},
									},
									{
										Config: &atc.GetStep{
											Name: "input4",
										},
									},
								},
							},
							{
								Name: "some-upstream-job",
							},
						},
						Resources: atc.ResourceConfigs{
							{Name: "input1", Type: "some-type", Source: atc.Source{"some": "source-1"}},
							{Name: "input2", Type: "some-type", Source: atc.Source{"some": "source-2"}},
							{Name: "input3", Type: "some-type", Source: atc.Source{"some": "source-3"}},
							{Name: "input4", Type: "some-type", Source: atc.Source{"some": "source-4"}},
						},
					}

					pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(2), false)
					Expect(err).ToNot(HaveOccurred())

					err = job.SaveNextInputMapping(db.InputMapping{
						"input2": db.InputResult{
							ResolveError: "resolve error",
						},
					}, false)
					Expect(err).NotTo(HaveOccurred())

					expectedBuildPrep.Inputs = map[string]db.BuildPreparationStatus{
						"input2": db.BuildPreparationStatusBlocking,
					}
					expectedBuildPrep.InputsSatisfied = db.BuildPreparationStatusBlocking
					expectedBuildPrep.MissingInputReasons = db.MissingInputReasons{
						"input2": "resolve error",
					}
				})

				It("returns blocking inputs satisfied", func() {
					buildPrep, found, err := build.Preparation()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(buildPrep).To(Equal(expectedBuildPrep))
				})
			})

			Context("when some inputs are missing", func() {
				BeforeEach(func() {
					pipelineConfig := atc.Config{
						Jobs: atc.JobConfigs{
							{
								Name: "some-job",
								PlanSequence: []atc.Step{
									{
										Config: &atc.GetStep{
											Name: "input1",
										},
									},
									{
										Config: &atc.GetStep{
											Name: "input2",
										},
									},
								},
							},
						},
						Resources: atc.ResourceConfigs{
							{Name: "input1", Type: "some-type", Source: atc.Source{"some": "source-1"}},
							{Name: "input2", Type: "some-type", Source: atc.Source{"some": "source-2"}},
						},
					}

					pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(2), false)
					Expect(err).ToNot(HaveOccurred())

					setupTx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())

					brt := db.BaseResourceType{
						Name: "some-type",
					}

					_, err = brt.FindOrCreate(setupTx, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(setupTx.Commit()).To(Succeed())

					job, found, err := pipeline.Job("some-job")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					resource1, found, err := pipeline.Resource("input1")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					resourceConfig1, err := resource1.SetResourceConfig(atc.Source{"some": "source-1"}, atc.VersionedResourceTypes{})
					Expect(err).NotTo(HaveOccurred())

					err = resourceConfig1.SaveVersions(nil, []atc.Version{{"version": "v1"}})
					Expect(err).NotTo(HaveOccurred())

					version, found, err := resourceConfig1.FindVersion(atc.Version{"version": "v1"})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					updated, err := resourceConfig1.UpdateLastCheckEndTime()
					Expect(err).NotTo(HaveOccurred())
					Expect(updated).To(BeTrue())

					reloaded, err := resource1.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(reloaded).To(BeTrue())

					lastCheckEndTime := resource1.LastCheckEndTime()
					Expect(lastCheckEndTime.IsZero()).To(BeFalse())

					err = job.SaveNextInputMapping(db.InputMapping{
						"input1": db.InputResult{
							Input: &db.AlgorithmInput{
								AlgorithmVersion: db.AlgorithmVersion{
									Version:    db.ResourceVersion(convertToMD5(atc.Version(version.Version()))),
									ResourceID: resource1.ID(),
								},
								FirstOccurrence: true,
							},
							PassedBuildIDs: []int{},
						},
					}, false)
					Expect(err).NotTo(HaveOccurred())

					expectedBuildPrep.Inputs = map[string]db.BuildPreparationStatus{
						"input1": db.BuildPreparationStatusNotBlocking,
						"input2": db.BuildPreparationStatusBlocking,
					}
					expectedBuildPrep.InputsSatisfied = db.BuildPreparationStatusBlocking
					expectedBuildPrep.MissingInputReasons = db.MissingInputReasons{
						"input2": "input is not included in resolved candidates",
					}
				})

				It("returns blocking inputs satisfied", func() {
					buildPrep, found, err := build.Preparation()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(buildPrep).To(Equal(expectedBuildPrep))
				})
			})
		})
	})

	Describe("AdoptInputsAndPipes", func() {
		var build, otherBuild, otherBuild2 db.Build
		var pipeline db.Pipeline
		var job, otherJob db.Job
		var buildInputs, expectedBuildInputs []db.BuildInput
		var adoptFound, reloadFound bool
		var err error

		BeforeEach(func() {
			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
					{
						Name: "some-other-job",
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					},
					{
						Name:   "some-other-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "other-source"},
					},
				},
			}

			var err error
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())

			var found bool
			job, found, err = pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err = job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			otherJob, found, err = pipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			otherBuild, err = otherJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			otherBuild2, err = otherJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			buildInputs, adoptFound, err = build.AdoptInputsAndPipes()
			Expect(err).ToNot(HaveOccurred())

			reloadFound, err = build.Reload()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when inputs are determined", func() {
			var (
				resource, otherResource                       db.Resource
				versions, otherVersions                       []atc.ResourceVersion
				resourceConfigScope, otherResourceConfigScope db.ResourceConfigScope
			)

			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "some-type",
				}

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				var found bool
				resource, found, err = pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				resourceConfigScope, err = resource.SetResourceConfig(atc.Source{"some": "source"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				err = resourceConfigScope.SaveVersions(nil, []atc.Version{
					{"version": "v1"},
					{"version": "v2"},
					{"version": "v3"},
				})
				Expect(err).NotTo(HaveOccurred())

				otherResource, found, err = pipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				otherResourceConfigScope, err = otherResource.SetResourceConfig(atc.Source{"some": "other-source"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				err = otherResourceConfigScope.SaveVersions(nil, []atc.Version{atc.Version{"version": "v1"}})
				Expect(err).ToNot(HaveOccurred())

				versions, _, found, err = resource.Versions(db.Page{Limit: 3}, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				otherVersions, _, found, err = otherResource.Versions(db.Page{Limit: 3}, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				// Set up existing build inputs
				err = job.SaveNextInputMapping(db.InputMapping{
					"some-input-0": db.InputResult{
						Input: &db.AlgorithmInput{
							AlgorithmVersion: db.AlgorithmVersion{
								Version:    db.ResourceVersion(convertToMD5(versions[2].Version)),
								ResourceID: resource.ID(),
							},
							FirstOccurrence: false,
						},
						PassedBuildIDs: []int{otherBuild2.ID()},
					}}, true)
				Expect(err).ToNot(HaveOccurred())

				Expect(build.InputsReady()).To(BeFalse())
			})

			Context("when version history is reset", func() {
				BeforeEach(func() {
					_, err = resource.SetResourceConfig(atc.Source{"some": "some-other-source"}, atc.VersionedResourceTypes{})
					Expect(err).ToNot(HaveOccurred())
				})

				It("set resolve error of that input", func() {
					Expect(adoptFound).To(BeFalse())
					Expect(reloadFound).To(BeTrue())

					nextBuildInputs, err := job.GetNextBuildInputs()
					Expect(err).ToNot(HaveOccurred())
					Expect(len(nextBuildInputs)).To(Equal(1))
					Expect(nextBuildInputs[0].ResolveError).To(Equal("chosen version of input some-input-0 not available"))
				})
			})

			Context("when inputs are not changed", func() {
				BeforeEach(func() {
					var found bool
					_, found, err = build.AdoptInputsAndPipes()
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					reloaded, err := build.Reload()
					Expect(err).ToNot(HaveOccurred())
					Expect(reloaded).To(BeTrue())

					Expect(build.InputsReady()).To(BeTrue())

					// Set up new next build inputs
					inputVersions := db.InputMapping{
						"some-input-1": db.InputResult{
							Input: &db.AlgorithmInput{
								AlgorithmVersion: db.AlgorithmVersion{
									Version:    db.ResourceVersion(convertToMD5(versions[0].Version)),
									ResourceID: resource.ID(),
								},
								FirstOccurrence: false,
							},
							PassedBuildIDs: []int{otherBuild.ID()},
						},
						"some-input-2": db.InputResult{
							Input: &db.AlgorithmInput{
								AlgorithmVersion: db.AlgorithmVersion{
									Version:    db.ResourceVersion(convertToMD5(versions[1].Version)),
									ResourceID: resource.ID(),
								},
								FirstOccurrence: false,
							},
							PassedBuildIDs: []int{},
						},
						"some-input-3": db.InputResult{
							Input: &db.AlgorithmInput{
								AlgorithmVersion: db.AlgorithmVersion{
									Version:    db.ResourceVersion(convertToMD5(otherVersions[0].Version)),
									ResourceID: otherResource.ID(),
								},
								FirstOccurrence: true,
							},
							PassedBuildIDs: []int{otherBuild.ID()},
						},
					}

					err = job.SaveNextInputMapping(inputVersions, true)
					Expect(err).ToNot(HaveOccurred())

					expectedBuildInputs = []db.BuildInput{
						{
							Name:            "some-input-1",
							ResourceID:      resource.ID(),
							Version:         versions[0].Version,
							FirstOccurrence: false,
						},
						{
							Name:            "some-input-2",
							ResourceID:      resource.ID(),
							Version:         versions[1].Version,
							FirstOccurrence: false,
						},
						{
							Name:            "some-input-3",
							ResourceID:      otherResource.ID(),
							Version:         otherVersions[0].Version,
							FirstOccurrence: true,
						},
					}

				})

				It("deletes existing build inputs and moves next build inputs to build inputs and next build pipes to build pipes", func() {
					Expect(adoptFound).To(BeTrue())
					Expect(reloadFound).To(BeTrue())

					Expect(buildInputs).To(ConsistOf(expectedBuildInputs))

					buildPipes, err := versionsDB.LatestBuildPipes(ctx, build.ID())
					Expect(err).ToNot(HaveOccurred())
					Expect(buildPipes[otherJob.ID()]).To(Equal(db.BuildCursor{
						ID: otherBuild.ID(),
					}))

					Expect(build.InputsReady()).To(BeTrue())
				})
			})
		})

		Context("when inputs are not determined", func() {
			BeforeEach(func() {
				err := job.SaveNextInputMapping(db.InputMapping{
					"some-input-1": db.InputResult{
						ResolveError: "errored",
					}}, false)
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not move build inputs and pipes", func() {
				Expect(adoptFound).To(BeFalse())
				Expect(reloadFound).To(BeTrue())

				Expect(buildInputs).To(BeNil())

				buildPipes, err := versionsDB.LatestBuildPipes(ctx, build.ID())
				Expect(err).ToNot(HaveOccurred())
				Expect(buildPipes).To(HaveLen(0))

				Expect(build.InputsReady()).To(BeFalse())
			})
		})
	})

	Describe("AdoptRerunInputsAndPipes", func() {
		var build, retriggerBuild, otherBuild db.Build
		var pipeline db.Pipeline
		var job, otherJob db.Job
		var buildInputs, expectedBuildInputs []db.BuildInput
		var adoptFound, reloadFound bool
		var err error

		BeforeEach(func() {
			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
					{
						Name: "some-other-job",
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					},
					{
						Name:   "some-other-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "other-source"},
					},
				},
			}

			var err error
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())

			var found bool
			otherJob, found, err = pipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			otherBuild, err = otherJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			job, found, err = pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err = job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			retriggerBuild, err = job.RerunBuild(build)
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			buildInputs, adoptFound, err = retriggerBuild.AdoptRerunInputsAndPipes()
			Expect(err).ToNot(HaveOccurred())

			reloadFound, err = build.Reload()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the build to retrigger of has inputs and pipes", func() {
			var (
				resource, otherResource                       db.Resource
				versions, otherVersions                       []atc.ResourceVersion
				resourceConfigScope, otherResourceConfigScope db.ResourceConfigScope
			)

			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "some-type",
				}

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				var found bool
				resource, found, err = pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				resourceConfigScope, err = resource.SetResourceConfig(atc.Source{"some": "source"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				err = resourceConfigScope.SaveVersions(nil, []atc.Version{
					{"version": "v1"},
					{"version": "v2"},
					{"version": "v3"},
				})
				Expect(err).NotTo(HaveOccurred())

				otherResource, found, err = pipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				otherResourceConfigScope, err = otherResource.SetResourceConfig(atc.Source{"some": "other-source"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				err = otherResourceConfigScope.SaveVersions(nil, []atc.Version{atc.Version{"version": "v1"}})
				Expect(err).ToNot(HaveOccurred())

				versions, _, found, err = resource.Versions(db.Page{Limit: 3}, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				otherVersions, _, found, err = otherResource.Versions(db.Page{Limit: 3}, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				// Set up existing build inputs
				err = job.SaveNextInputMapping(db.InputMapping{
					"some-input-0": db.InputResult{
						Input: &db.AlgorithmInput{
							AlgorithmVersion: db.AlgorithmVersion{
								Version:    db.ResourceVersion(convertToMD5(versions[2].Version)),
								ResourceID: resource.ID(),
							},
							FirstOccurrence: false,
						},
						PassedBuildIDs: []int{otherBuild.ID()},
					}}, true)
				Expect(err).ToNot(HaveOccurred())

				Expect(retriggerBuild.InputsReady()).To(BeFalse())

				_, found, err = retriggerBuild.AdoptInputsAndPipes()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				reloaded, err := retriggerBuild.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(reloaded).To(BeTrue())
				Expect(retriggerBuild.InputsReady()).To(BeTrue())

				// Set up new next build inputs
				inputVersions := db.InputMapping{
					"some-input-1": db.InputResult{
						Input: &db.AlgorithmInput{
							AlgorithmVersion: db.AlgorithmVersion{
								Version:    db.ResourceVersion(convertToMD5(versions[0].Version)),
								ResourceID: resource.ID(),
							},
							FirstOccurrence: false,
						},
						PassedBuildIDs: []int{otherBuild.ID()},
					},
					"some-input-2": db.InputResult{
						Input: &db.AlgorithmInput{
							AlgorithmVersion: db.AlgorithmVersion{
								Version:    db.ResourceVersion(convertToMD5(versions[1].Version)),
								ResourceID: resource.ID(),
							},
							FirstOccurrence: false,
						},
						PassedBuildIDs: []int{},
					},
					"some-input-3": db.InputResult{
						Input: &db.AlgorithmInput{
							AlgorithmVersion: db.AlgorithmVersion{
								Version:    db.ResourceVersion(convertToMD5(otherVersions[0].Version)),
								ResourceID: otherResource.ID(),
							},
							FirstOccurrence: true,
						},
						PassedBuildIDs: []int{otherBuild.ID()},
					},
				}

				err = job.SaveNextInputMapping(inputVersions, true)
				Expect(err).ToNot(HaveOccurred())

				_, found, err = build.AdoptInputsAndPipes()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				expectedBuildInputs = []db.BuildInput{
					{
						Name:            "some-input-1",
						ResourceID:      resource.ID(),
						Version:         versions[0].Version,
						FirstOccurrence: false,
					},
					{
						Name:            "some-input-2",
						ResourceID:      resource.ID(),
						Version:         versions[1].Version,
						FirstOccurrence: false,
					},
					{
						Name:            "some-input-3",
						ResourceID:      otherResource.ID(),
						Version:         otherVersions[0].Version,
						FirstOccurrence: false,
					},
				}
			})

			Context("when version history is reset", func() {
				BeforeEach(func() {
					_, err = otherResource.SetResourceConfig(atc.Source{"some": "some-other-source"}, atc.VersionedResourceTypes{})
					Expect(err).ToNot(HaveOccurred())
				})

				It("set resolve error of that input", func() {
					Expect(adoptFound).To(BeFalse())
					Expect(reloadFound).To(BeTrue())

					nextBuildInputs, err := job.GetNextBuildInputs()
					Expect(err).ToNot(HaveOccurred())
					Expect(len(nextBuildInputs)).To(Equal(3))
					Expect(nextBuildInputs).To(ContainElements(db.BuildInput{
						Name:            "some-input-3",
						ResourceID:      otherResource.ID(),
						Version:         nil,
						FirstOccurrence: true,
						ResolveError:    "chosen version of input some-input-3 not available",
					}))
				})
			})

			It("deletes existing build inputs and uses the build inputs and pipes of the build to retrigger off of as it's own build inputs but sets first occurrence to false", func() {
				Expect(adoptFound).To(BeTrue())
				Expect(reloadFound).To(BeTrue())

				Expect(buildInputs).To(ConsistOf(expectedBuildInputs))

				buildPipes, err := versionsDB.LatestBuildPipes(ctx, retriggerBuild.ID())
				Expect(err).ToNot(HaveOccurred())
				Expect(buildPipes).To(HaveLen(1))
				Expect(buildPipes[otherJob.ID()]).To(Equal(db.BuildCursor{
					ID: otherBuild.ID(),
				}))

				reloaded, err := retriggerBuild.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(reloaded).To(BeTrue())
				Expect(retriggerBuild.InputsReady()).To(BeTrue())
			})
		})

		Context("when the build to retrigger off of does not have inputs or pipes", func() {
			It("does not move build inputs and pipes", func() {
				Expect(adoptFound).To(BeFalse())
				Expect(reloadFound).To(BeTrue())

				Expect(buildInputs).To(BeNil())

				buildPipes, err := versionsDB.LatestBuildPipes(ctx, build.ID())
				Expect(err).ToNot(HaveOccurred())
				Expect(buildPipes).To(HaveLen(0))

				reloaded, err := retriggerBuild.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(reloaded).To(BeTrue())
				Expect(retriggerBuild.InputsReady()).To(BeFalse())
			})
		})
	})

	Describe("ResourcesChecked", func() {
		var build db.Build
		var resourceConfigScope1, resourceConfigScope2 db.ResourceConfigScope
		var checked bool
		var resource2 db.Resource

		BeforeEach(func() {
			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name: "some-resource",
								},
							},
							{
								Config: &atc.GetStep{
									Name: "some-other-resource",
								},
							},
						},
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					},
					{
						Name:   "some-other-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "other-source"},
					},
				},
			}

			var err error
			pipeline, _, err := team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())

			var found bool
			job, found, err := pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "some-type",
			}

			_, err = brt.FindOrCreate(setupTx, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			resource1, found, err := pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resourceConfigScope1, err = resource1.SetResourceConfig(atc.Source{"some": "source"}, atc.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			resource2, found, err = pipeline.Resource("some-other-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resourceConfigScope2, err = resource2.SetResourceConfig(atc.Source{"some": "other-source"}, atc.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			build, err = job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error
			checked, err = build.ResourcesChecked()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when all the resources in the build has been checked", func() {
			BeforeEach(func() {
				updated, err := resourceConfigScope1.UpdateLastCheckEndTime()
				Expect(err).ToNot(HaveOccurred())
				Expect(updated).To(BeTrue())

				updated, err = resourceConfigScope2.UpdateLastCheckEndTime()
				Expect(err).ToNot(HaveOccurred())
				Expect(updated).To(BeTrue())
			})

			It("returns true", func() {
				Expect(checked).To(BeTrue())
			})
		})

		Context("when a the resource in the build has not been checked", func() {
			It("returns false", func() {
				Expect(checked).To(BeFalse())
			})
		})

		Context("when a pinned resource in the build has not been checked", func() {
			BeforeEach(func() {
				updated, err := resourceConfigScope1.UpdateLastCheckEndTime()
				Expect(err).ToNot(HaveOccurred())
				Expect(updated).To(BeTrue())

				err = resourceConfigScope2.SaveVersions(nil, []atc.Version{
					{"ver": "1"},
				})
				Expect(err).ToNot(HaveOccurred())

				rcv, found, err := resourceConfigScope2.FindVersion(atc.Version{"ver": "1"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				found, err = resource2.PinVersion(rcv.ID())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("returns true", func() {
				Expect(checked).To(BeTrue())
			})
		})
	})

	Describe("SavePipeline", func() {
		It("saves the parent job and build ids", func() {
			By("creating a build")
			build, err := defaultJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			By("saving a pipeline with the build")
			pipeline, _, err := build.SavePipeline("other-pipeline", build.TeamID(), atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-base-resource-type",
						Source: atc.Source{
							"some": "source",
						},
					},
				},
				ResourceTypes: atc.ResourceTypes{
					{
						Name: "some-type",
						Type: "some-base-resource-type",
						Source: atc.Source{
							"some-type": "source",
						},
					},
				},
			}, db.ConfigVersion(0), false)
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.ParentJobID()).To(Equal(build.JobID()))
			Expect(pipeline.ParentBuildID()).To(Equal(build.ID()))
		})

		It("only saves the pipeline if it is the latest build", func() {
			By("creating two builds")
			buildOne, err := defaultJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())
			buildTwo, err := defaultJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			By("saving a pipeline with the second build")
			pipeline, _, err := buildTwo.SavePipeline("other-pipeline", buildTwo.TeamID(), atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-base-resource-type",
						Source: atc.Source{
							"some": "source",
						},
					},
				},
				ResourceTypes: atc.ResourceTypes{
					{
						Name: "some-type",
						Type: "some-base-resource-type",
						Source: atc.Source{
							"some-type": "source",
						},
					},
				},
			}, db.ConfigVersion(0), false)
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.ParentJobID()).To(Equal(buildTwo.JobID()))
			Expect(pipeline.ParentBuildID()).To(Equal(buildTwo.ID()))

			By("saving a pipeline with the first build")
			pipeline, _, err = buildOne.SavePipeline("other-pipeline", buildOne.TeamID(), atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-base-resource-type",
						Source: atc.Source{
							"some": "source",
						},
					},
				},
				ResourceTypes: atc.ResourceTypes{
					{
						Name: "some-type",
						Type: "some-base-resource-type",
						Source: atc.Source{
							"some-type": "source",
						},
					},
				},
			}, pipeline.ConfigVersion(), false)
			Expect(err).To(Equal(db.ErrSetByNewerBuild))
		})

		Context("a pipeline is previously saved by team.SavePipeline", func() {
			It("the parent job and build ID are updated", func() {
				By("creating a build")
				build, err := defaultJob.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				By("re-saving the default pipeline with the build")
				pipeline, _, err := build.SavePipeline("default-pipeline", build.TeamID(), defaultPipelineConfig, db.ConfigVersion(1), false)
				Expect(err).ToNot(HaveOccurred())
				Expect(pipeline.ParentJobID()).To(Equal(build.JobID()))
				Expect(pipeline.ParentBuildID()).To(Equal(build.ID()))
			})
		})
	})
})

func envelope(ev atc.Event) event.Envelope {
	payload, err := json.Marshal(ev)
	Expect(err).ToNot(HaveOccurred())

	data := json.RawMessage(payload)

	return event.Envelope{
		Event:   ev.EventType(),
		Version: ev.Version(),
		Data:    &data,
	}
}

func convertToMD5(version atc.Version) string {
	versionJSON, err := json.Marshal(version)
	Expect(err).ToNot(HaveOccurred())

	hasher := md5.New()
	hasher.Write([]byte(versionJSON))
	return hex.EncodeToString(hasher.Sum(nil))
}

func getJobBuildID(col string, jobID int) int {
	var result int

	err := psql.Select(col).
		From("jobs").
		Where(sq.Eq{"id": jobID}).
		RunWith(dbConn).
		QueryRow().
		Scan(&result)
	Expect(err).ToNot(HaveOccurred())

	return result
}
