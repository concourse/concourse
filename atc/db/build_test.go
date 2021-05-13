package db_test

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/dummy"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbtest"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/vars"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gocache "github.com/patrickmn/go-cache"
)

var _ = Describe("Build", func() {
	var (
		versionsDB db.VersionsDB

		// XXX(dbtests): remove these
		team  db.Team
		build db.Build
		job   db.Job

		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()

		versionsDB = db.NewVersionsDB(dbConn, 100, gocache.New(10*time.Second, 10*time.Second))

		var err error
		var found bool
		team, err = teamFactory.CreateTeam(atc.Team{Name: "some-team"})
		Expect(err).ToNot(HaveOccurred())

		pipelineConfig := atc.Config{
			Jobs: atc.JobConfigs{
				{Name: "some-job"},
			},
		}

		pipeline, _, err := team.SavePipeline(atc.PipelineRef{Name: "some-build-pipeline"}, pipelineConfig, db.ConfigVersion(1), false)
		Expect(err).ToNot(HaveOccurred())

		job, found, err = pipeline.Job("some-job")
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		build, err = job.CreateBuild(defaultBuildCreatedBy)
		Expect(err).NotTo(HaveOccurred())
	})

	It("created_by should be set", func() {
		Expect(build.CreatedBy()).ToNot(BeNil())
		Expect(*build.CreatedBy()).To(Equal(defaultBuildCreatedBy))
	})

	It("has no plan on creation", func() {
		build, err := team.CreateOneOffBuild()
		Expect(err).ToNot(HaveOccurred())
		Expect(build.HasPlan()).To(BeFalse())
	})

	It("create_time is current time", func(){
		Expect(build.CreateTime()).To(BeTemporally("<", time.Now(), 1*time.Second))
	})

	Describe("LagerData", func() {
		var build db.Build

		var data lager.Data

		JustBeforeEach(func() {
			data = build.LagerData()
		})

		Context("for a one-off build", func() {
			BeforeEach(func() {
				var err error
				build, err = team.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())
			})

			It("includes build and team info", func() {
				Expect(data).To(Equal(lager.Data{
					"build_id": build.ID(),
					"build":    build.Name(),
					"team":     team.Name(),
				}))
			})
		})

		Context("for a job build", func() {
			BeforeEach(func() {
				var err error
				build, err = defaultJob.CreateBuild(defaultBuildCreatedBy)
				Expect(err).ToNot(HaveOccurred())
				Expect(build.CreatedBy()).ToNot(BeNil())
				Expect(*build.CreatedBy()).To(Equal(defaultBuildCreatedBy))
			})

			It("includes build, team, pipeline, and job info", func() {
				Expect(data).To(Equal(lager.Data{
					"build_id": build.ID(),
					"build":    build.Name(),
					"team":     build.TeamName(),
					"pipeline": build.PipelineName(),
					"job":      defaultJob.Name(),
				}))
			})
		})

		Context("for a resource build", func() {
			BeforeEach(func() {
				var err error
				var created bool
				build, created, err = defaultResource.CreateBuild(context.TODO(), false, atc.Plan{})
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeTrue())
			})

			It("includes build, team, and pipeline", func() {
				Expect(data).To(Equal(lager.Data{
					"build_id": build.ID(),
					"build":    build.Name(),
					"team":     build.TeamName(),
					"pipeline": build.PipelineName(),
					"resource": defaultResource.Name(),
				}))
			})
		})

		Context("for a resource type build", func() {
			BeforeEach(func() {
				var err error
				var created bool
				build, created, err = defaultResourceType.CreateBuild(context.TODO(), false, atc.Plan{})
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeTrue())
			})

			It("includes build, team, and pipeline", func() {
				Expect(data).To(Equal(lager.Data{
					"build_id":      build.ID(),
					"build":         build.Name(),
					"team":          build.TeamName(),
					"pipeline":      build.PipelineName(),
					"resource_type": defaultResourceType.Name(),
				}))
			})
		})
	})

	Describe("SyslogTag", func() {
		var build db.Build

		var originID event.OriginID = "some-origin"
		var tag string

		JustBeforeEach(func() {
			tag = build.SyslogTag(originID)
		})

		Context("for a one-off build", func() {
			BeforeEach(func() {
				var err error
				build, err = team.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())
			})

			It("includes build and team info", func() {
				Expect(tag).To(Equal(fmt.Sprintf("%s/%d/%s", team.Name(), build.ID(), originID)))
			})
		})

		Context("for a job build", func() {
			BeforeEach(func() {
				var err error
				build, err = defaultJob.CreateBuild(defaultBuildCreatedBy)
				Expect(err).ToNot(HaveOccurred())
			})

			It("includes build, team, pipeline, and job info", func() {
				Expect(tag).To(Equal(fmt.Sprintf("%s/%s/%s/%s/%s", defaultJob.TeamName(), defaultJob.PipelineName(), defaultJob.Name(), build.Name(), originID)))
			})
		})

		Context("for a resource build", func() {
			BeforeEach(func() {
				var err error
				var created bool
				build, created, err = defaultResource.CreateBuild(context.TODO(), false, atc.Plan{})
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeTrue())
			})

			It("includes build, team, and pipeline", func() {
				Expect(tag).To(Equal(fmt.Sprintf("%s/%s/%s/%d/%s", defaultResource.TeamName(), defaultResource.PipelineName(), defaultResource.Name(), build.ID(), originID)))
			})
		})

		Context("for a resource type build", func() {
			BeforeEach(func() {
				var err error
				var created bool
				build, created, err = defaultResourceType.CreateBuild(context.TODO(), false, atc.Plan{})
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeTrue())
			})

			It("includes build, team, and pipeline", func() {
				Expect(tag).To(Equal(fmt.Sprintf("%s/%s/%s/%d/%s", defaultResourceType.TeamName(), defaultResourceType.PipelineName(), defaultResourceType.Name(), build.ID(), originID)))
			})
		})
	})

	Describe("TracingAttrs", func() {
		var build db.Build

		var attrs tracing.Attrs

		JustBeforeEach(func() {
			attrs = build.TracingAttrs()
		})

		Context("for a one-off build", func() {
			BeforeEach(func() {
				var err error
				build, err = team.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())
			})

			It("includes build and team info", func() {
				Expect(attrs).To(Equal(tracing.Attrs{
					"build_id":  strconv.Itoa(build.ID()),
					"build":     build.Name(),
					"team_name": team.Name(),
				}))
			})
		})

		Context("for a job build", func() {
			BeforeEach(func() {
				var err error
				build, err = defaultJob.CreateBuild(defaultBuildCreatedBy)
				Expect(err).ToNot(HaveOccurred())
			})

			It("includes build, team, pipeline, and job info", func() {
				Expect(attrs).To(Equal(tracing.Attrs{
					"build_id":  strconv.Itoa(build.ID()),
					"build":     build.Name(),
					"team_name": build.TeamName(),
					"pipeline":  build.PipelineName(),
					"job":       defaultJob.Name(),
				}))
			})
		})

		Context("for a resource build", func() {
			BeforeEach(func() {
				var err error
				var created bool
				build, created, err = defaultResource.CreateBuild(context.TODO(), false, atc.Plan{})
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeTrue())
			})

			It("includes build, team, and pipeline", func() {
				Expect(attrs).To(Equal(tracing.Attrs{
					"build_id":  strconv.Itoa(build.ID()),
					"build":     build.Name(),
					"team_name": build.TeamName(),
					"pipeline":  build.PipelineName(),
					"resource":  defaultResource.Name(),
				}))
			})
		})

		Context("for a resource type build", func() {
			BeforeEach(func() {
				var err error
				var created bool
				build, created, err = defaultResourceType.CreateBuild(context.TODO(), false, atc.Plan{})
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeTrue())
			})

			It("includes build, team, and pipeline", func() {
				Expect(attrs).To(Equal(tracing.Attrs{
					"build_id":      strconv.Itoa(build.ID()),
					"build":         build.Name(),
					"team_name":     build.TeamName(),
					"pipeline":      build.PipelineName(),
					"resource_type": defaultResourceType.Name(),
				}))
			})
		})
	})

	Describe("Reload", func() {
		It("updates the model", func() {
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
			Expect(build.IsDrained()).To(BeFalse())
		})

		It("has drain set to true after a drain and a reload", func() {
			err := build.SetDrained(true)
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
				}, "0")))
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
		var scenario *dbtest.Scenario
		var build db.Build
		var expectedOutputs []db.AlgorithmVersion

		BeforeEach(func() {
			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name:     "input-1",
									Resource: "some-resource",
								},
							},
							{
								Config: &atc.GetStep{
									Name:     "input-2",
									Resource: "some-other-resource",
								},
							},
							{
								Config: &atc.GetStep{
									Name:     "input-3",
									Resource: "some-resource",
								},
							},
							{
								Config: &atc.GetStep{
									Name:     "input-4",
									Resource: "some-resource",
								},
							},
							{
								Config: &atc.PutStep{
									Name:     "output-1",
									Resource: "some-resource",
								},
							},
							{
								Config: &atc.PutStep{
									Name:     "output-2",
									Resource: "some-resource",
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
						Type:   dbtest.BaseResourceType,
						Source: atc.Source{"some": "source"},
					},
					{
						Name:   "some-other-resource",
						Type:   dbtest.BaseResourceType,
						Source: atc.Source{"some": "other-source"},
					},
				},
			}

			scenario = dbtest.Setup(
				builder.WithPipeline(pipelineConfig),
				builder.WithResourceVersions(
					"some-resource",
					atc.Version{"ver": "1"},
					atc.Version{"ver": "2"},
				),
				builder.WithResourceVersions(
					"some-other-resource",
					atc.Version{"ver": "1"},
					atc.Version{"ver": "2"},
					atc.Version{"ver": "3"},
				),
				builder.WithJobBuild(&build, "some-job", dbtest.JobInputs{
					{
						Name:    "input-1",
						Version: atc.Version{"ver": "1"},
					},
					{
						Name:    "input-2",
						Version: atc.Version{"ver": "3"},
					},
					{
						Name:    "input-3",
						Version: atc.Version{"ver": "2"},
					},
					{
						Name:    "input-4",
						Version: atc.Version{"ver": "2"},
					},
				}, dbtest.JobOutputs{
					"output-1": atc.Version{"ver": "2"},
					"output-2": atc.Version{"ver": "3"},
				}),
			)

			Expect(build.Finish(db.BuildStatusSucceeded)).To(Succeed())

			expectedOutputs = []db.AlgorithmVersion{
				{
					Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "1"})),
					ResourceID: scenario.Resource("some-resource").ID(),
				},
				{
					Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "3"})),
					ResourceID: scenario.Resource("some-other-resource").ID(),
				},
				{
					Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "2"})),
					ResourceID: scenario.Resource("some-resource").ID(),
				},
				{
					Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "3"})),
					ResourceID: scenario.Resource("some-resource").ID(),
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
			}, "0")))
		})

		It("updates build status", func() {
			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(build.Status()).To(Equal(db.BuildStatusSucceeded))
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

		Context("rerunning a build", func() {
			var (
				job db.Job

				pdBuild, pdBuild2, rrBuild db.Build
				err                        error
				latestCompletedBuildCol    = "latest_completed_build_id"
				nextBuildCol               = "next_build_id"
				transitionBuildCol         = "transition_build_id"
			)

			BeforeEach(func() {
				job = scenario.Job("some-job")
			})

			Context("when there is a pending build that is not a rerun", func() {
				BeforeEach(func() {
					pdBuild, err = job.CreateBuild(defaultBuildCreatedBy)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("when rerunning the latest completed build", func() {
					BeforeEach(func() {
						rrBuild, err = job.RerunBuild(build, defaultBuildCreatedBy)
						Expect(err).NotTo(HaveOccurred())
					})

					It("created_by should be set", func() {
						Expect(rrBuild.CreatedBy()).ToNot(BeNil())
						Expect(*rrBuild.CreatedBy()).To(Equal(defaultBuildCreatedBy))
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
							pdBuild2, err = job.CreateBuild(defaultBuildCreatedBy)
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
						rrBuild, err = job.RerunBuild(pdBuild, defaultBuildCreatedBy)
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

					It("created_by should be set", func() {
						Expect(rrBuild.CreatedBy()).ToNot(BeNil())
						Expect(*rrBuild.CreatedBy()).To(Equal(defaultBuildCreatedBy))
					})

					Context("when rerunning the rerun build", func() {
						var rrBuild2 db.Build

						BeforeEach(func() {
							err = rrBuild.Finish(db.BuildStatusSucceeded)
							Expect(err).NotTo(HaveOccurred())

							rrBuild2, err = job.RerunBuild(rrBuild, defaultBuildCreatedBy)
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

						rrBuild, err = job.RerunBuild(build, defaultBuildCreatedBy)
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

					It("created_by should be set", func() {
						Expect(rrBuild.CreatedBy()).ToNot(BeNil())
						Expect(*rrBuild.CreatedBy()).To(Equal(defaultBuildCreatedBy))
					})
				})
			})
		})

		Context("when requesting schedule", func() {
			It("request schedule on the downstream job", func() {
				job := scenario.Job("some-job")
				downstreamJob := scenario.Job("downstream-job")

				newBuild, err := job.CreateBuild(defaultBuildCreatedBy)
				Expect(err).NotTo(HaveOccurred())
				Expect(newBuild.CreatedBy()).ToNot(BeNil())
				Expect(*newBuild.CreatedBy()).To(Equal(defaultBuildCreatedBy))

				requestedSchedule := downstreamJob.ScheduleRequestedTime()

				err = newBuild.Finish(db.BuildStatusSucceeded)
				Expect(err).NotTo(HaveOccurred())

				found, err := downstreamJob.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(downstreamJob.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule))
			})

			It("do not request schedule on jobs that are not directly downstream", func() {
				job := scenario.Job("some-job")
				noRequestJob := scenario.Job("no-request-job")

				newBuild, err := job.CreateBuild(defaultBuildCreatedBy)
				Expect(err).NotTo(HaveOccurred())
				Expect(newBuild.CreatedBy()).ToNot(BeNil())
				Expect(*newBuild.CreatedBy()).To(Equal(defaultBuildCreatedBy))

				requestedSchedule := noRequestJob.ScheduleRequestedTime()

				err = newBuild.Finish(db.BuildStatusSucceeded)
				Expect(err).NotTo(HaveOccurred())

				found, err := noRequestJob.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(noRequestJob.ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule))
			})
		})

		Context("archiving pipelines", func() {
			var childPipeline db.Pipeline

			BeforeEach(func() {
				By("creating a child pipeline")
				build, _ := defaultJob.CreateBuild(defaultBuildCreatedBy)
				childPipeline, _, _ = build.SavePipeline(atc.PipelineRef{Name: "child1-pipeline"}, defaultTeam.ID(), defaultPipelineConfig, db.ConfigVersion(0), false)
				build.Finish(db.BuildStatusSucceeded)

				childPipeline.Reload()
				Expect(childPipeline.Archived()).To(BeFalse())
			})

			Context("build is successful", func() {
				It("archives pipelines no longer set by the job", func() {
					By("no longer setting the child pipeline")
					build2, _ := defaultJob.CreateBuild(defaultBuildCreatedBy)
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
							build, _ := job.CreateBuild(defaultBuildCreatedBy)
							childPipeline, _, _ = build.SavePipeline(atc.PipelineRef{Name: "child-pipeline-" + strconv.Itoa(i)}, defaultTeam.ID(), defaultPipelineConfig, db.ConfigVersion(0), false)
							build.Finish(db.BuildStatusSucceeded)
							childPipelines = append(childPipelines, childPipeline)
						}

						By("parent pipeline no longer sets child pipeline in most recent build")
						build, _ := defaultJob.CreateBuild(defaultBuildCreatedBy)
						build.Finish(db.BuildStatusSucceeded)

						for _, pipeline := range childPipelines {
							pipeline.Reload()
							Expect(pipeline.Archived()).To(BeTrue())
						}

					})
				})

				Context("when the pipeline is not set by build", func() {
					It("never gets archived", func() {
						build, _ := defaultJob.CreateBuild(defaultBuildCreatedBy)
						teamPipeline, _, _ := defaultTeam.SavePipeline(atc.PipelineRef{Name: "team-pipeline"}, defaultPipelineConfig, db.ConfigVersion(0), false)
						build.Finish(db.BuildStatusSucceeded)

						teamPipeline.Reload()
						Expect(teamPipeline.Archived()).To(BeFalse())
					})
				})
			})
			Context("build is not successful", func() {
				It("does not archive pipelines", func() {
					By("no longer setting the child pipeline")
					build2, _ := defaultJob.CreateBuild(defaultBuildCreatedBy)
					build2.Finish(db.BuildStatusFailed)

					childPipeline.Reload()
					Expect(childPipeline.Archived()).To(BeFalse())
				})
			})
		})
	})

	Describe("Variables", func() {
		var (
			globalSecrets creds.Secrets
			varSourcePool creds.VarSourcePool
		)

		BeforeEach(func() {
			globalSecrets = &dummy.Secrets{StaticVariables: vars.StaticVariables{"foo": "bar"}}

			credentialManagement := creds.CredentialManagementConfig{
				RetryConfig: creds.SecretRetryConfig{
					Attempts: 5,
					Interval: time.Second,
				},
				CacheConfig: creds.SecretCacheConfig{
					Enabled:          true,
					Duration:         time.Minute,
					DurationNotFound: time.Minute,
					PurgeInterval:    time.Minute * 10,
				},
			}
			varSourcePool = creds.NewVarSourcePool(logger, credentialManagement, 1*time.Minute, 1*time.Second, clock.NewClock())
		})

		Context("when the build is a one-off build", func() {
			var build db.Build

			BeforeEach(func() {
				var err error
				build, err = defaultTeam.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())
			})

			It("fetches from the global secrets", func() {
				v, err := build.Variables(logger, globalSecrets, varSourcePool)
				Expect(err).ToNot(HaveOccurred())

				val, found, err := v.Get(vars.Reference{Path: "foo"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(val).To(Equal("bar"))
			})
		})

		Context("when the build is a job build", func() {
			var build db.Build

			BeforeEach(func() {
				config := defaultPipelineConfig
				config.VarSources = append(config.VarSources, atc.VarSourceConfig{
					Name: "some-source",
					Type: "dummy",
					Config: map[string]interface{}{
						"vars": map[string]interface{}{"baz": "caz"},
					},
				})

				pipeline, _, err := defaultTeam.SavePipeline(defaultPipelineRef, config, defaultPipeline.ConfigVersion(), false)
				Expect(err).ToNot(HaveOccurred())

				job, found, err := pipeline.Job(defaultJob.Name())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err = job.CreateBuild(defaultBuildCreatedBy)
				Expect(err).ToNot(HaveOccurred())
			})

			It("fetches from the global secrets", func() {
				v, err := build.Variables(logger, globalSecrets, varSourcePool)
				Expect(err).ToNot(HaveOccurred())

				val, found, err := v.Get(vars.Reference{Path: "foo"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(val).To(Equal("bar"))
			})

			It("fetches from the var sources", func() {
				v, err := build.Variables(logger, globalSecrets, varSourcePool)
				Expect(err).ToNot(HaveOccurred())

				val, found, err := v.Get(vars.Reference{Source: "some-source", Path: "baz"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(val).To(Equal("caz"))
			})
		})
	})

	Describe("Abort", func() {
		JustBeforeEach(func() {
			err := build.MarkAsAborted()
			Expect(err).NotTo(HaveOccurred())

			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		It("updates aborted to true", func() {
			Expect(build.IsAborted()).To(BeTrue())
		})

		Context("request job rescheudle", func() {
			JustBeforeEach(func() {
				found, err := job.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			Context("when build is in pending state", func() {
				BeforeEach(func() {
					time.Sleep(1 * time.Second)
				})

				It("requests the job to reschedule immediately", func() {
					Expect(job.ScheduleRequestedTime()).Should(BeTemporally("~", time.Now(), time.Second))
				})
			})

			Context("when build is not in pending state", func() {
				var firstRequestTime time.Time

				BeforeEach(func() {
					firstRequestTime = time.Now()

					time.Sleep(1 * time.Second)

					err := build.Finish(db.BuildStatusFailed)
					Expect(err).NotTo(HaveOccurred())

					found, err := build.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
				})

				It("does not request reschedule", func() {
					Expect(job.ScheduleRequestedTime()).Should(BeTemporally("~", firstRequestTime, time.Second))
				})
			})
		})
	})

	Describe("Events", func() {
		It("saves and emits status events", func() {
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
			}, "0")))

			By("emitting a status event when finished")
			err = build.Finish(db.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			found, err = build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(events.Next()).To(Equal(envelope(event.Status{
				Status: atc.StatusSucceeded,
				Time:   build.EndTime().Unix(),
			}, "1")))

			By("ending the stream when finished")
			_, err = events.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))
		})

		It("emits pre-bigint migration events", func() {
			started, err := build.Start(atc.Plan{})
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			found, err := build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = dbConn.Exec(`UPDATE build_events SET build_id_old = build_id, build_id = NULL WHERE build_id = $1`, build.ID())
			Expect(err).NotTo(HaveOccurred())

			events, err := build.Events(0)
			Expect(err).NotTo(HaveOccurred())

			defer db.Close(events)
			Expect(events.Next()).To(Equal(envelope(event.Status{
				Status: atc.StatusStarted,
				Time:   build.StartTime().Unix(),
			}, "0")))
		})
	})

	Describe("SaveEvent", func() {
		It("saves and propagates events correctly", func() {
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
			}, "0")))

			err = build.SaveEvent(event.Log{
				Payload: "log",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(events.Next()).To(Equal(envelope(event.Log{
				Payload: "log",
			}, "1")))

			By("allowing you to subscribe from an offset")
			eventsFrom1, err := build.Events(1)
			Expect(err).NotTo(HaveOccurred())

			defer db.Close(eventsFrom1)

			Expect(eventsFrom1.Next()).To(Equal(envelope(event.Log{
				Payload: "log",
			}, "1")))

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
			}, "2"))))

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
		var pipelineConfig atc.Config

		var scenario *dbtest.Scenario
		var build db.Build

		var outputVersion atc.Version

		BeforeEach(func() {
			atc.EnableGlobalResources = true

			pipelineConfig = atc.Config{
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
						Name: "some-other-job",
						PlanSequence: []atc.Step{
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
						Type:   dbtest.BaseResourceType,
						Source: atc.Source{"some": "source"},
					},
					{
						Name:   "some-other-resource",
						Type:   dbtest.BaseResourceType,
						Source: atc.Source{"some": "other-source"},
					},
				},
			}

			scenario = dbtest.Setup(
				builder.WithPipeline(pipelineConfig),
				builder.WithResourceVersions("some-resource", atc.Version{"some": "version"}),
				builder.WithResourceVersions("some-other-resource", atc.Version{"some": "other-version"}),
				builder.WithJobBuild(&build, "some-job", dbtest.JobInputs{
					{
						Name:    "some-resource",
						Version: atc.Version{"some": "version"},
					},
				}, dbtest.JobOutputs{}),
			)

			outputVersion = atc.Version{"some": "new-version"}
		})

		JustBeforeEach(func() {
			err := build.SaveOutput(
				dbtest.BaseResourceType,
				atc.Source{"some": "source"},
				atc.VersionedResourceTypes{},
				outputVersion,
				[]db.ResourceConfigMetadataField{
					{
						Name:  "meta",
						Value: "data",
					},
				},
				"output-name",
				"some-resource",
			)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			atc.EnableGlobalResources = false
		})

		It("should set the resource's config scope", func() {
			resource, found, err := scenario.Pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resource.ResourceConfigScopeID()).ToNot(BeZero())
		})

		Context("when the version does not exist", func() {
			It("can save a build's output", func() {
				rcv := scenario.ResourceVersion("some-resource", outputVersion)
				Expect(rcv.Version()).To(Equal(db.Version(outputVersion)))

				_, buildOutputs, err := build.Resources()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(buildOutputs)).To(Equal(1))
				Expect(buildOutputs[0].Name).To(Equal("output-name"))
				Expect(buildOutputs[0].Version).To(Equal(outputVersion))
			})

			Context("with a job in a separate team downstream of the same resource config", func() {
				var otherScenario *dbtest.Scenario

				var beforeTime, otherJobBeforeTime time.Time
				var otherTeamBeforeTime, otherTeamOtherJobBeforeTime time.Time

				BeforeEach(func() {
					otherScenario = dbtest.Setup(
						builder.WithTeam("other-team"),
						builder.WithPipeline(pipelineConfig),
						builder.WithResourceVersions("some-resource", atc.Version{"some": "version"}),
						builder.WithResourceVersions("some-other-resource", atc.Version{"some": "other-version"}),
					)

					beforeTime = scenario.Job("some-job").ScheduleRequestedTime()
					otherTeamBeforeTime = otherScenario.Job("some-job").ScheduleRequestedTime()

					otherJobBeforeTime = scenario.Job("some-other-job").ScheduleRequestedTime()
					otherTeamOtherJobBeforeTime = otherScenario.Job("some-other-job").ScheduleRequestedTime()
				})

				It("requests schedule on jobs which use the same config", func() {
					Expect(scenario.Job("some-job").ScheduleRequestedTime()).To(BeTemporally(">", beforeTime))
					Expect(otherScenario.Job("some-job").ScheduleRequestedTime()).To(BeTemporally(">", otherTeamBeforeTime))

					Expect(scenario.Job("some-other-job").ScheduleRequestedTime()).To(BeTemporally("==", otherJobBeforeTime))
					Expect(otherScenario.Job("some-other-job").ScheduleRequestedTime()).To(BeTemporally("==", otherTeamOtherJobBeforeTime))
				})
			})
		})

		Context("when the version already exists", func() {
			var rcv db.ResourceConfigVersion

			BeforeEach(func() {
				scenario.Run(
					builder.WithResourceVersions("some-resource", outputVersion),
				)

				rcv = scenario.ResourceVersion("some-resource", outputVersion)
			})

			It("does not increment the check order", func() {
				newRCV := scenario.ResourceVersion("some-resource", outputVersion)
				Expect(newRCV.CheckOrder()).To(Equal(rcv.CheckOrder()))
			})

			Context("with a job in a separate team downstream of the same resource config", func() {
				var otherScenario *dbtest.Scenario

				var beforeTime, otherJobBeforeTime time.Time
				var otherTeamBeforeTime, otherTeamOtherJobBeforeTime time.Time

				BeforeEach(func() {
					otherScenario = dbtest.Setup(
						builder.WithTeam("other-team"),
						builder.WithPipeline(pipelineConfig),
						builder.WithResourceVersions("some-resource", atc.Version{"some": "version"}),
						builder.WithResourceVersions("some-other-resource", atc.Version{"some": "other-version"}),
					)

					beforeTime = scenario.Job("some-job").ScheduleRequestedTime()
					otherTeamBeforeTime = otherScenario.Job("some-job").ScheduleRequestedTime()

					otherJobBeforeTime = scenario.Job("some-other-job").ScheduleRequestedTime()
					otherTeamOtherJobBeforeTime = otherScenario.Job("some-other-job").ScheduleRequestedTime()
				})

				It("does not request schedule on jobs which use the same config", func() {
					Expect(scenario.Job("some-job").ScheduleRequestedTime()).To(BeTemporally("==", beforeTime))
					Expect(otherScenario.Job("some-job").ScheduleRequestedTime()).To(BeTemporally("==", otherTeamBeforeTime))

					Expect(scenario.Job("some-other-job").ScheduleRequestedTime()).To(BeTemporally("==", otherJobBeforeTime))
					Expect(otherScenario.Job("some-other-job").ScheduleRequestedTime()).To(BeTemporally("==", otherTeamOtherJobBeforeTime))
				})
			})
		})
	})

	Describe("Resources", func() {
		var (
			scenario      *dbtest.Scenario
			build         db.Build
			inputResource db.Resource
		)

		BeforeEach(func() {
			pipelineConfig := atc.Config{
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
							{
								Config: &atc.PutStep{
									Name: "some-resource",
								},
							},
							{
								Config: &atc.PutStep{
									Name: "some-other-resource",
								},
							},
						},
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name:   "some-resource",
						Type:   dbtest.BaseResourceType,
						Source: atc.Source{"some": "source-1"},
					},
					{
						Name:   "some-other-resource",
						Type:   dbtest.BaseResourceType,
						Source: atc.Source{"some": "source-2"},
					},
					{
						Name:   "some-unused-resource",
						Type:   dbtest.BaseResourceType,
						Source: atc.Source{"some": "source-3"},
					},
				},
			}

			scenario = dbtest.Setup(
				builder.WithPipeline(pipelineConfig),
				builder.WithResourceVersions(
					"some-resource",
					atc.Version{"ver": "1"},
					atc.Version{"ver": "2"},
				),
				builder.WithJobBuild(&build, "some-job", dbtest.JobInputs{
					{
						Name:            "some-input",
						Version:         atc.Version{"ver": "1"},
						FirstOccurrence: true,
					},
				}, dbtest.JobOutputs{
					"some-resource":       atc.Version{"ver": "2"},
					"some-other-resource": atc.Version{"ver": "not-checked"},
				}),
			)

			inputResource = scenario.Resource("some-resource")
		})

		It("returns build inputs and outputs", func() {
			inputs, outputs, err := build.Resources()
			Expect(err).NotTo(HaveOccurred())

			Expect(inputs).To(ConsistOf([]db.BuildInput{
				{
					Name:            "some-input",
					Version:         atc.Version{"ver": "1"},
					ResourceID:      inputResource.ID(),
					FirstOccurrence: true,
				},
			}))

			Expect(outputs).To(ConsistOf([]db.BuildOutput{
				{
					Name:    "some-resource",
					Version: atc.Version{"ver": "2"},
				},
				{
					Name:    "some-other-resource",
					Version: atc.Version{"ver": "not-checked"},
				},
			}))
		})

		Context("when the first occurrence is empty", func() {
			BeforeEach(func() {
				res, err := psql.Update("build_resource_config_version_inputs").
					Set("first_occurrence", nil).
					Where(sq.Eq{
						"build_id":    build.ID(),
						"resource_id": inputResource.ID(),
						"version_md5": convertToMD5(atc.Version{"ver": "1"}),
					}).
					RunWith(dbConn).
					Exec()
				Expect(err).NotTo(HaveOccurred())
				rows, err := res.RowsAffected()
				Expect(err).NotTo(HaveOccurred())
				Expect(rows).To(Equal(int64(1)))
			})

			It("determines the first occurrence to be true", func() {
				inputs, _, err := build.Resources()
				Expect(err).NotTo(HaveOccurred())
				Expect(inputs).To(ConsistOf([]db.BuildInput{
					{
						Name:            "some-input",
						Version:         atc.Version{"ver": "1"},
						ResourceID:      inputResource.ID(),
						FirstOccurrence: true,
					},
				}))
			})

			Context("when the a build with those inputs already exist", func() {
				var newBuild db.Build

				BeforeEach(func() {
					scenario.Run(
						builder.WithJobBuild(&newBuild, "some-job", dbtest.JobInputs{
							{
								Name:            "some-input",
								Version:         atc.Version{"ver": "1"},
								FirstOccurrence: true,
							},
						}, dbtest.JobOutputs{
							"some-resource":       atc.Version{"ver": "2"},
							"some-other-resource": atc.Version{"ver": "not-checked"},
						}),
					)

					res, err := psql.Update("build_resource_config_version_inputs").
						Set("first_occurrence", nil).
						Where(sq.Eq{
							"build_id":    newBuild.ID(),
							"resource_id": inputResource.ID(),
							"version_md5": convertToMD5(atc.Version{"ver": "1"}),
						}).
						RunWith(dbConn).
						Exec()
					Expect(err).NotTo(HaveOccurred())

					rows, err := res.RowsAffected()
					Expect(err).NotTo(HaveOccurred())
					Expect(rows).To(Equal(int64(1)))
				})

				It("determines the first occurrence to be false", func() {
					inputs, _, err := newBuild.Resources()
					Expect(err).NotTo(HaveOccurred())
					Expect(inputs).To(ConsistOf([]db.BuildInput{
						{
							Name:            "some-input",
							Version:         atc.Version{"ver": "1"},
							ResourceID:      inputResource.ID(),
							FirstOccurrence: false,
						},
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
				createdPipeline, _, err = team.SavePipeline(atc.PipelineRef{Name: "some-pipeline"}, atc.Config{
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

				build, err = job.CreateBuild(defaultBuildCreatedBy)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the correct pipeline", func() {
				Expect(found).To(BeTrue())
				Expect(foundPipeline.Name()).To(Equal(createdPipeline.Name()))
			})
		})
	})

	Describe("Preparation", func() {
		var (
			scenario *dbtest.Scenario
			job      db.Job

			expectedBuildPrep db.BuildPreparation
		)

		BeforeEach(func() {
			scenario = dbtest.Setup(
				builder.WithPipeline(atc.Config{
					Resources: atc.ResourceConfigs{
						{
							Name: "some-resource",
							Type: dbtest.BaseResourceType,
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
				}),
				builder.WithResourceVersions("some-resource", atc.Version{"version": "some-version"}),
				builder.WithPendingJobBuild(&build, "some-job"),
			)

			job = scenario.Job("some-job")

			expectedBuildPrep = db.BuildPreparation{
				BuildID:             build.ID(),
				PausedPipeline:      db.BuildPreparationStatusNotBlocking,
				PausedJob:           db.BuildPreparationStatusNotBlocking,
				MaxRunningBuilds:    db.BuildPreparationStatusNotBlocking,
				Inputs:              map[string]db.BuildPreparationStatus{},
				InputsSatisfied:     db.BuildPreparationStatusNotBlocking,
				MissingInputReasons: db.MissingInputReasons{},
			}
		})

		Context("when inputs are satisfied", func() {
			BeforeEach(func() {
				scenario.Run(
					builder.WithNextInputMapping("some-job", dbtest.JobInputs{
						{
							Name:    "some-input",
							Version: atc.Version{"version": "some-version"},
						},
					}),
				)
			})

			Context("when resource check finished after build created", func() {
				BeforeEach(func() {
					scenario.Run(
						// don't save any versions, just bump the last check timestamp
						builder.WithResourceVersions("some-resource"),
					)

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
						err := scenario.Pipeline.Pause()
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
						err := scenario.Job("some-job").Pause()
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
					var secondBuild db.Build

					BeforeEach(func() {
						scenario.Run(
							builder.WithPendingJobBuild(&secondBuild, "some-job"),
							// don't save any versions, just bump the last check timestamp
							builder.WithResourceVersions("some-resource"),
						)

						scheduled, err := job.ScheduleBuild(build)
						Expect(err).ToNot(HaveOccurred())
						Expect(scheduled).To(BeTrue())

						scheduled, err = job.ScheduleBuild(secondBuild)
						Expect(err).ToNot(HaveOccurred())
						Expect(scheduled).To(BeFalse())

						expectedBuildPrep.BuildID = secondBuild.ID()
						expectedBuildPrep.MaxRunningBuilds = db.BuildPreparationStatusBlocking
					})

					It("returns build preparation with max in flight reached", func() {
						buildPrep, found, err := secondBuild.Preparation()
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
						Expect(buildPrep).To(Equal(expectedBuildPrep))
					})

					Context("when max running builds is de-reached", func() {
						BeforeEach(func() {
							err := build.Finish(db.BuildStatusSucceeded)
							Expect(err).NotTo(HaveOccurred())

							scheduled, err := job.ScheduleBuild(secondBuild)
							Expect(err).ToNot(HaveOccurred())
							Expect(scheduled).To(BeTrue())

							expectedBuildPrep.MaxRunningBuilds = db.BuildPreparationStatusNotBlocking
						})

						It("returns build preparation with max in flight not reached", func() {
							buildPrep, found, err := secondBuild.Preparation()
							Expect(err).NotTo(HaveOccurred())
							Expect(found).To(BeTrue())
							Expect(buildPrep).To(Equal(expectedBuildPrep))
						})
					})
				})
			})

			Context("when no resource check finished after build created", func() {
				BeforeEach(func() {
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
				scenario.Run(
					builder.WithNextInputMapping("some-job", dbtest.JobInputs{
						{
							Name:         "some-input",
							ResolveError: "resolve error",
						},
					}),
				)

				expectedBuildPrep.Inputs = map[string]db.BuildPreparationStatus{
					"some-input": db.BuildPreparationStatusBlocking,
				}
				expectedBuildPrep.InputsSatisfied = db.BuildPreparationStatusBlocking
				expectedBuildPrep.MissingInputReasons = db.MissingInputReasons{
					"some-input": "resolve error",
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
				scenario.Run(
					builder.WithNextInputMapping("some-job", dbtest.JobInputs{
						{
							Name:    "some-input",
							Version: atc.Version{"some": "version"},
						},
					}),

					// checked after build creation
					builder.WithResourceVersions("some-resource"),

					// add another input
					builder.WithPipeline(atc.Config{
						Resources: atc.ResourceConfigs{
							{
								Name: "some-resource",
								Type: dbtest.BaseResourceType,
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
									{
										Config: &atc.GetStep{
											Name:     "some-other-input",
											Resource: "some-resource",
										},
									},
								},
							},
						},
					}),
				)

				expectedBuildPrep.Inputs = map[string]db.BuildPreparationStatus{
					"some-input":       db.BuildPreparationStatusNotBlocking,
					"some-other-input": db.BuildPreparationStatusBlocking,
				}
				expectedBuildPrep.InputsSatisfied = db.BuildPreparationStatusBlocking
				expectedBuildPrep.MissingInputReasons = db.MissingInputReasons{
					"some-other-input": "input is not included in resolved candidates",
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

	Describe("AdoptInputsAndPipes", func() {
		var scenario *dbtest.Scenario

		BeforeEach(func() {
			scenario = dbtest.Setup(
				builder.WithPipeline(atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "upstream-job",
							PlanSequence: []atc.Step{
								{
									Config: &atc.GetStep{
										Name:     "some-input",
										Resource: "some-resource",
									},
								},
							},
						},
						{
							Name: "downstream-job",
							PlanSequence: []atc.Step{
								{
									Config: &atc.GetStep{
										Name:     "some-input",
										Resource: "some-resource",
										Passed:   []string{"upstream-job"},
									},
								},
								{
									Config: &atc.GetStep{
										Name:     "some-other-input",
										Resource: "some-other-resource",
									},
								},
							},
						},
					},
					Resources: atc.ResourceConfigs{
						{
							Name:   "some-resource",
							Type:   dbtest.BaseResourceType,
							Source: atc.Source{"some": "source"},
						},
						{
							Name:   "some-other-resource",
							Type:   dbtest.BaseResourceType,
							Source: atc.Source{"some": "other-source"},
						},
					},
				}),
				builder.WithResourceVersions("some-resource",
					atc.Version{"version": "v1"},
					atc.Version{"version": "v2"},
					atc.Version{"version": "v3"},
				),
				builder.WithResourceVersions("some-other-resource",
					atc.Version{"version": "v1"},
				),
			)
		})

		Describe("adopting inputs for an upstream job", func() {
			var upstreamBuild db.Build

			BeforeEach(func() {
				scenario.Run(
					builder.WithPendingJobBuild(&upstreamBuild, "upstream-job"),
				)
			})

			Context("for valid versions", func() {
				BeforeEach(func() {
					scenario.Run(
						builder.WithNextInputMapping("upstream-job", dbtest.JobInputs{
							{
								Name:    "some-input",
								Version: atc.Version{"version": "v3"},
							},
						}),
					)
				})

				It("adopts the next inputs and pipes", func() {
					inputs, adopted, err := upstreamBuild.AdoptInputsAndPipes()
					Expect(err).ToNot(HaveOccurred())
					Expect(adopted).To(BeTrue())
					Expect(inputs).To(ConsistOf([]db.BuildInput{
						{
							Name:            "some-input",
							ResourceID:      scenario.Resource("some-resource").ID(),
							Version:         atc.Version{"version": "v3"},
							FirstOccurrence: false,
						},
					}))

					Expect(upstreamBuild.InputsReady()).To(BeFalse())
					found, err := upstreamBuild.Reload()
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(upstreamBuild.InputsReady()).To(BeTrue())
				})

				Context("followed by a downstream job", func() {
					var downstreamBuild db.Build

					BeforeEach(func() {
						_, adopted, err := upstreamBuild.AdoptInputsAndPipes()
						Expect(err).ToNot(HaveOccurred())
						Expect(adopted).To(BeTrue())

						Expect(upstreamBuild.Finish(db.BuildStatusSucceeded)).To(Succeed())

						scenario.Run(
							builder.WithPendingJobBuild(&downstreamBuild, "downstream-job"),
							builder.WithNextInputMapping("downstream-job", dbtest.JobInputs{
								{
									Name:         "some-input",
									Version:      atc.Version{"version": "v3"},
									PassedBuilds: []db.Build{upstreamBuild},
								},
								{
									Name:            "some-other-input",
									Version:         atc.Version{"version": "v1"},
									FirstOccurrence: true,
								},
							}),
						)
					})

					It("adopts the next inputs and pipes referencing the upstream build", func() {
						inputs, adopted, err := downstreamBuild.AdoptInputsAndPipes()
						Expect(err).ToNot(HaveOccurred())
						Expect(adopted).To(BeTrue())
						Expect(inputs).To(ConsistOf([]db.BuildInput{
							{
								Name:            "some-input",
								ResourceID:      scenario.Resource("some-resource").ID(),
								Version:         atc.Version{"version": "v3"},
								FirstOccurrence: false,
							},
							{
								Name:            "some-other-input",
								ResourceID:      scenario.Resource("some-other-resource").ID(),
								Version:         atc.Version{"version": "v1"},
								FirstOccurrence: true,
							},
						}))

						Expect(downstreamBuild.InputsReady()).To(BeFalse())
						found, err := downstreamBuild.Reload()
						Expect(err).ToNot(HaveOccurred())
						Expect(found).To(BeTrue())
						Expect(downstreamBuild.InputsReady()).To(BeTrue())

						buildPipes, err := versionsDB.LatestBuildPipes(ctx, downstreamBuild.ID())
						Expect(err).ToNot(HaveOccurred())
						Expect(buildPipes[scenario.Job("upstream-job").ID()]).To(Equal(db.BuildCursor{
							ID: upstreamBuild.ID(),
						}))
					})
				})
			})

			Context("for bogus versions", func() {
				BeforeEach(func() {
					scenario.Run(
						builder.WithNextInputMapping("upstream-job", dbtest.JobInputs{
							{
								Name:    "some-input",
								Version: atc.Version{"version": "bogus"},
							},
						}),
					)
				})

				It("set resolve error of that input", func() {
					buildInputs, adopted, err := upstreamBuild.AdoptInputsAndPipes()
					Expect(err).ToNot(HaveOccurred())
					Expect(adopted).To(BeFalse())
					Expect(buildInputs).To(BeNil())

					nextBuildInputs, err := scenario.Job("upstream-job").GetNextBuildInputs()
					Expect(err).ToNot(HaveOccurred())
					Expect(len(nextBuildInputs)).To(Equal(1))
					Expect(nextBuildInputs[0].ResolveError).To(Equal("chosen version of input some-input not available"))
				})
			})

			Context("when inputs are not determined", func() {
				BeforeEach(func() {
					scenario.Run(
						builder.WithNextInputMapping("upstream-job", dbtest.JobInputs{
							{
								Name:         "some-input",
								Version:      atc.Version{"version": "bogus"},
								ResolveError: "errored",
							},
						}),
					)
				})

				It("does not adopt next inputs and pipes", func() {
					buildInputs, adopted, err := upstreamBuild.AdoptInputsAndPipes()
					Expect(err).ToNot(HaveOccurred())
					Expect(adopted).To(BeFalse())
					Expect(buildInputs).To(BeNil())

					buildPipes, err := versionsDB.LatestBuildPipes(ctx, upstreamBuild.ID())
					Expect(err).ToNot(HaveOccurred())
					Expect(buildPipes).To(HaveLen(0))

					Expect(upstreamBuild.InputsReady()).To(BeFalse())
					found, err := upstreamBuild.Reload()
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(upstreamBuild.InputsReady()).To(BeFalse())
				})
			})
		})
	})

	Describe("AdoptRerunInputsAndPipes", func() {
		var scenario *dbtest.Scenario
		var pipelineConfig atc.Config

		var upstreamBuild, downstreamBuild, retriggerBuild db.Build
		var buildInputs, expectedBuildInputs []db.BuildInput
		var adoptFound bool

		BeforeEach(func() {
			pipelineConfig = atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "upstream-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name:     "some-input",
									Resource: "some-resource",
								},
							},
						},
					},
					{
						Name: "downstream-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name:     "some-input",
									Resource: "some-resource",
									Passed:   []string{"upstream-job"},
								},
							},
							{
								Config: &atc.GetStep{
									Name:     "some-other-input",
									Resource: "some-other-resource",
								},
							},
						},
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name:   "some-resource",
						Type:   dbtest.BaseResourceType,
						Source: atc.Source{"some": "source"},
					},
					{
						Name:   "some-other-resource",
						Type:   dbtest.BaseResourceType,
						Source: atc.Source{"some": "other-source"},
					},
				},
			}

			scenario = dbtest.Setup(
				builder.WithPipeline(pipelineConfig),
				builder.WithResourceVersions("some-resource",
					atc.Version{"version": "v1"},
					atc.Version{"version": "v2"},
					atc.Version{"version": "v3"},
				),
				builder.WithResourceVersions("some-other-resource",
					atc.Version{"version": "v1"},
				),
				builder.WithJobBuild(&upstreamBuild, "upstream-job", dbtest.JobInputs{
					{
						Name:    "some-input",
						Version: atc.Version{"version": "v3"},
					},
				}, dbtest.JobOutputs{}),
				builder.WithPendingJobBuild(&downstreamBuild, "downstream-job"),
			)

			var err error
			retriggerBuild, err = job.RerunBuild(downstreamBuild, defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())
			Expect(retriggerBuild.CreatedBy()).NotTo(BeNil())
			Expect(*retriggerBuild.CreatedBy()).To(Equal(defaultBuildCreatedBy))
		})

		JustBeforeEach(func() {
			var err error
			buildInputs, adoptFound, err = retriggerBuild.AdoptRerunInputsAndPipes()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the build to retrigger has inputs and pipes", func() {
			BeforeEach(func() {
				scenario.Run(
					builder.WithNextInputMapping("downstream-job", dbtest.JobInputs{
						{
							Name:            "some-input",
							Version:         atc.Version{"version": "v3"},
							PassedBuilds:    []db.Build{upstreamBuild},
							FirstOccurrence: true,
						},
						{
							Name:            "some-other-input",
							Version:         atc.Version{"version": "v1"},
							FirstOccurrence: true,
						},
					}),
				)

				_, found, err := downstreamBuild.AdoptInputsAndPipes()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				expectedBuildInputs = []db.BuildInput{
					{
						Name:            "some-input",
						ResourceID:      scenario.Resource("some-resource").ID(),
						Version:         atc.Version{"version": "v3"},
						FirstOccurrence: false,
					},
					{
						Name:            "some-other-input",
						ResourceID:      scenario.Resource("some-other-resource").ID(),
						Version:         atc.Version{"version": "v1"},
						FirstOccurrence: false,
					},
				}
			})

			It("build inputs and pipes of the build to retrigger off of as it's own build inputs but sets first occurrence to false", func() {
				Expect(adoptFound).To(BeTrue())

				Expect(buildInputs).To(ConsistOf(expectedBuildInputs))

				buildPipes, err := versionsDB.LatestBuildPipes(ctx, retriggerBuild.ID())
				Expect(err).ToNot(HaveOccurred())
				Expect(buildPipes).To(HaveLen(1))
				Expect(buildPipes[scenario.Job("upstream-job").ID()]).To(Equal(db.BuildCursor{
					ID: upstreamBuild.ID(),
				}))

				reloaded, err := retriggerBuild.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(reloaded).To(BeTrue())
				Expect(retriggerBuild.InputsReady()).To(BeTrue())
			})

			Context("when the version becomes unavailable", func() {
				BeforeEach(func() {
					pipelineConfig.Resources[0].Source = atc.Source{"some": "new-source"}

					scenario.Run(
						builder.WithPipeline(pipelineConfig),
						builder.WithResourceVersions("some-resource", atc.Version{"some": "new-version"}),
					)
				})

				It("fails to adopt", func() {
					Expect(adoptFound).To(BeFalse())
				})

				It("aborts the build", func() {
					reloaded, err := retriggerBuild.Reload()
					Expect(err).ToNot(HaveOccurred())
					Expect(reloaded).To(BeTrue())
					Expect(retriggerBuild.IsAborted()).To(BeTrue())
				})
			})
		})

		Context("when the build to retrigger off of does not have inputs or pipes", func() {
			It("does not move build inputs and pipes", func() {
				Expect(adoptFound).To(BeFalse())

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
		var scenario *dbtest.Scenario

		var build db.Build
		var checked bool

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
						Type:   dbtest.BaseResourceType,
						Source: atc.Source{"some": "source"},
					},
					{
						Name:   "some-other-resource",
						Type:   dbtest.BaseResourceType,
						Source: atc.Source{"some": "other-source"},
					},
				},
			}

			scenario = dbtest.Setup(
				builder.WithPipeline(pipelineConfig),
				builder.WithResourceVersions("some-resource", atc.Version{"some": "version"}),
				builder.WithResourceVersions("some-other-resource", atc.Version{"some": "other-version"}),
				builder.WithPendingJobBuild(&build, "some-job"),
			)
		})

		JustBeforeEach(func() {
			var err error
			checked, err = build.ResourcesChecked()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when all the resources in the build has been checked", func() {
			BeforeEach(func() {
				scenario.Run(
					builder.WithResourceVersions("some-resource"),
					builder.WithResourceVersions("some-other-resource"),
				)
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
				scenario.Run(
					builder.WithResourceVersions("some-resource"),
				)

				rcv := scenario.ResourceVersion("some-other-resource", atc.Version{"some": "other-version"})

				found, err := scenario.Resource("some-other-resource").PinVersion(rcv.ID())
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
			build, err := defaultJob.CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())

			By("saving a pipeline with the build")
			pipeline, _, err := build.SavePipeline(atc.PipelineRef{Name: "other-pipeline"}, build.TeamID(), atc.Config{
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
			buildOne, err := defaultJob.CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())
			buildTwo, err := defaultJob.CreateBuild(defaultBuildCreatedBy)
			Expect(err).ToNot(HaveOccurred())

			By("saving a pipeline with the second build")
			pipeline, _, err := buildTwo.SavePipeline(atc.PipelineRef{Name: "other-pipeline"}, buildTwo.TeamID(), atc.Config{
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
			_, _, err = buildOne.SavePipeline(atc.PipelineRef{Name: "other-pipeline"}, buildOne.TeamID(), atc.Config{
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
				build, err := defaultJob.CreateBuild(defaultBuildCreatedBy)
				Expect(err).ToNot(HaveOccurred())

				By("re-saving the default pipeline with the build")
				pipeline, _, err := build.SavePipeline(defaultPipelineRef, build.TeamID(), defaultPipelineConfig, db.ConfigVersion(1), false)
				Expect(err).ToNot(HaveOccurred())
				Expect(pipeline.ParentJobID()).To(Equal(build.JobID()))
				Expect(pipeline.ParentBuildID()).To(Equal(build.ID()))
			})
		})
	})
})

func envelope(ev atc.Event, eventID string) event.Envelope {
	payload, err := json.Marshal(ev)
	Expect(err).ToNot(HaveOccurred())

	data := json.RawMessage(payload)

	return event.Envelope{
		Event:   ev.EventType(),
		Version: ev.Version(),
		Data:    &data,
		EventID: eventID,
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
