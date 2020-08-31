package db_test

import (
	"context"
	"fmt"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/tracing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel/api/propagators"
)

var _ = Describe("Job", func() {
	var (
		job      db.Job
		pipeline db.Pipeline
		team     db.Team
	)

	BeforeEach(func() {
		var err error
		team, err = teamFactory.CreateTeam(atc.Team{Name: "some-team"})
		Expect(err).ToNot(HaveOccurred())

		var created bool
		pipeline, created, err = team.SavePipeline("fake-pipeline", atc.Config{
			Jobs: atc.JobConfigs{
				{
					Name: "some-job",

					Public: true,

					PlanSequence: []atc.Step{
						{
							Config: &atc.PutStep{
								Name: "some-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
							},
						},
						{
							Config: &atc.GetStep{
								Name:     "some-input",
								Resource: "some-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
								Passed:  []string{"job-1", "job-2"},
								Trigger: true,
							},
						},
						{
							Config: &atc.TaskStep{
								Name:       "some-task",
								Privileged: true,
								ConfigPath: "some/config/path.yml",
								Config: &atc.TaskConfig{
									RootfsURI: "some-image",
								},
							},
						},
					},
				},
				{
					Name: "some-other-job",
				},
				{
					Name:   "some-private-job",
					Public: false,
				},
				{
					Name: "other-serial-group-job",
				},
				{
					Name: "different-serial-group-job",
				},
				{
					Name: "job-1",
				},
				{
					Name: "job-2",
				},
				{
					Name:                 "non-triggerable-job",
					DisableManualTrigger: true,
				},
			},
			Resources: atc.ResourceConfigs{
				{
					Name: "some-resource",
					Type: "some-type",
				},
				{
					Name: "some-other-resource",
					Type: "some-type",
				},
			},
		}, db.ConfigVersion(0), false)
		Expect(err).ToNot(HaveOccurred())
		Expect(created).To(BeTrue())

		var found bool
		job, found, err = pipeline.Job("some-job")
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
	})

	Describe("Public", func() {
		Context("when the config has public set to true", func() {
			It("returns true", func() {
				Expect(job.Public()).To(BeTrue())
			})
		})

		Context("when the config has public set to false", func() {
			It("returns false", func() {
				privateJob, found, err := pipeline.Job("some-private-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(privateJob.Public()).To(BeFalse())
			})
		})

		Context("when the config does not have public set", func() {
			It("returns false", func() {
				otherJob, found, err := pipeline.Job("some-other-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(otherJob.Public()).To(BeFalse())
			})
		})
	})

	Describe("DisableManualTrigger", func() {
		Context("when the config has disable_manual_trigger set to true", func() {
			It("returns true", func() {
				nonTriggerableJob, found, err := pipeline.Job("non-triggerable-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(nonTriggerableJob.DisableManualTrigger()).To(BeTrue())
			})
		})

		Context("when the config does not have disable_manual_trigger set", func() {
			It("returns false", func() {
				otherJob, found, err := pipeline.Job("some-other-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(otherJob.DisableManualTrigger()).To(BeFalse())
			})
		})
	})

	Describe("Pause and Unpause", func() {
		var initialRequestedTime time.Time
		It("starts out as unpaused", func() {
			Expect(job.Paused()).To(BeFalse())
		})

		Context("when pausing job", func() {
			BeforeEach(func() {
				initialRequestedTime = job.ScheduleRequestedTime()

				err := job.Pause()
				Expect(err).ToNot(HaveOccurred())

				found, err := job.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("job is succesfully paused", func() {
				Expect(job.Paused()).To(BeTrue())
			})

			It("does not request schedule on job", func() {
				Expect(job.ScheduleRequestedTime()).Should(BeTemporally("==", initialRequestedTime))
			})
		})

		Context("when unpausing job", func() {
			BeforeEach(func() {
				initialRequestedTime = job.ScheduleRequestedTime()

				err := job.Unpause()
				Expect(err).ToNot(HaveOccurred())

				found, err := job.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("job is successfully unpaused", func() {
				Expect(job.Paused()).To(BeFalse())
			})

			It("requests schedule on job", func() {
				Expect(job.ScheduleRequestedTime()).Should(BeTemporally(">", initialRequestedTime))
			})
		})

	})

	Describe("FinishedAndNextBuild", func() {
		var otherPipeline db.Pipeline
		var otherJob db.Job

		BeforeEach(func() {
			var created bool
			var err error
			otherPipeline, created, err = team.SavePipeline("other-pipeline", atc.Config{
				Jobs: atc.JobConfigs{
					{Name: "some-job"},
				},
			}, db.ConfigVersion(0), false)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).To(BeTrue())

			var found bool
			otherJob, found, err = otherPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		It("can report a job's latest running and finished builds", func() {
			finished, next, err := job.FinishedAndNextBuild()
			Expect(err).NotTo(HaveOccurred())

			Expect(next).To(BeNil())
			Expect(finished).To(BeNil())

			finishedBuild, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			err = finishedBuild.Finish(db.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			otherFinishedBuild, err := otherJob.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			err = otherFinishedBuild.Finish(db.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			finished, next, err = job.FinishedAndNextBuild()
			Expect(err).NotTo(HaveOccurred())

			Expect(next).To(BeNil())
			Expect(finished.ID()).To(Equal(finishedBuild.ID()))

			nextBuild, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			started, err := nextBuild.Start(atc.Plan{})
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			otherNextBuild, err := otherJob.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			otherStarted, err := otherNextBuild.Start(atc.Plan{})
			Expect(err).NotTo(HaveOccurred())
			Expect(otherStarted).To(BeTrue())

			finished, next, err = job.FinishedAndNextBuild()
			Expect(err).NotTo(HaveOccurred())

			Expect(next.ID()).To(Equal(nextBuild.ID()))
			Expect(finished.ID()).To(Equal(finishedBuild.ID()))

			anotherRunningBuild, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			finished, next, err = job.FinishedAndNextBuild()
			Expect(err).NotTo(HaveOccurred())

			Expect(next.ID()).To(Equal(nextBuild.ID())) // not anotherRunningBuild
			Expect(finished.ID()).To(Equal(finishedBuild.ID()))

			started, err = anotherRunningBuild.Start(atc.Plan{})
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			finished, next, err = job.FinishedAndNextBuild()
			Expect(err).NotTo(HaveOccurred())

			Expect(next.ID()).To(Equal(nextBuild.ID())) // not anotherRunningBuild
			Expect(finished.ID()).To(Equal(finishedBuild.ID()))

			err = nextBuild.Finish(db.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			finished, next, err = job.FinishedAndNextBuild()
			Expect(err).NotTo(HaveOccurred())

			Expect(next.ID()).To(Equal(anotherRunningBuild.ID()))
			Expect(finished.ID()).To(Equal(nextBuild.ID()))
		})
	})

	Describe("UpdateFirstLoggedBuildID", func() {
		It("updates FirstLoggedBuildID on a job", func() {
			By("starting out as 0")
			Expect(job.FirstLoggedBuildID()).To(BeZero())

			By("increasing it to 57")
			err := job.UpdateFirstLoggedBuildID(57)
			Expect(err).NotTo(HaveOccurred())

			found, err := job.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(job.FirstLoggedBuildID()).To(Equal(57))

			By("not erroring when it's called with the same number")
			err = job.UpdateFirstLoggedBuildID(57)
			Expect(err).NotTo(HaveOccurred())

			By("erroring when the number decreases")
			err = job.UpdateFirstLoggedBuildID(56)
			Expect(err).To(Equal(db.FirstLoggedBuildIDDecreasedError{
				Job:   "some-job",
				OldID: 57,
				NewID: 56,
			}))
		})
	})

	Describe("Builds", func() {
		var (
			builds       [10]db.Build
			someJob      db.Job
			someOtherJob db.Job
		)

		BeforeEach(func() {
			for i := 0; i < 10; i++ {
				var found bool
				var err error
				someJob, found, err = pipeline.Job("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				someOtherJob, found, err = pipeline.Job("some-other-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err := someJob.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				_, err = someOtherJob.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				builds[i] = build
			}
		})

		Context("when there are no builds to be found", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := someOtherJob.Builds(db.Page{})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]db.Build{}))
				Expect(pagination).To(Equal(db.Pagination{}))
			})
		})

		Context("with no from/to", func() {
			It("returns the first page, with the given limit, and a next page", func() {
				buildsPage, pagination, err := someJob.Builds(db.Page{Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]db.Build{builds[9], builds[8]}))
				Expect(pagination.Newer).To(BeNil())
				Expect(pagination.Older).To(Equal(&db.Page{To: builds[7].ID(), Limit: 2}))
			})
		})

		Context("with a to that places it in the middle of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := someJob.Builds(db.Page{To: builds[6].ID(), Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]db.Build{builds[6], builds[5]}))
				Expect(pagination.Newer).To(Equal(&db.Page{From: builds[7].ID(), Limit: 2}))
				Expect(pagination.Older).To(Equal(&db.Page{To: builds[4].ID(), Limit: 2}))
			})
		})

		Context("with a to that places it at the end of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := someJob.Builds(db.Page{To: builds[1].ID(), Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]db.Build{builds[1], builds[0]}))
				Expect(pagination.Newer).To(Equal(&db.Page{From: builds[2].ID(), Limit: 2}))
				Expect(pagination.Older).To(BeNil())
			})
		})

		Context("with a from that places it in the middle of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := someJob.Builds(db.Page{From: builds[6].ID(), Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]db.Build{builds[7], builds[6]}))
				Expect(pagination.Newer).To(Equal(&db.Page{From: builds[8].ID(), Limit: 2}))
				Expect(pagination.Older).To(Equal(&db.Page{To: builds[5].ID(), Limit: 2}))
			})
		})

		Context("with a from that places it at the beginning of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				buildsPage, pagination, err := someJob.Builds(db.Page{From: builds[8].ID(), Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(buildsPage).To(Equal([]db.Build{builds[9], builds[8]}))
				Expect(pagination.Newer).To(BeNil())
				Expect(pagination.Older).To(Equal(&db.Page{To: builds[7].ID(), Limit: 2}))
			})
		})
	})

	Describe("BuildsWithTime", func() {

		var (
			pipeline db.Pipeline
			builds   = make([]db.Build, 4)
			job      db.Job
		)

		BeforeEach(func() {
			var (
				err   error
				found bool
			)

			config := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
					{
						Name: "some-other-job",
					},
				},
			}
			pipeline, _, err = team.SavePipeline("some-pipeline", config, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())

			job, found, err = pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			for i := range builds {
				builds[i], err = job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				buildStart := time.Date(2020, 11, i+1, 0, 0, 0, 0, time.UTC)
				_, err = dbConn.Exec("UPDATE builds SET start_time = to_timestamp($1) WHERE id = $2", buildStart.Unix(), builds[i].ID())
				Expect(err).NotTo(HaveOccurred())

				builds[i], found, err = job.Build(builds[i].Name())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			}
		})

		Context("when not providing boundaries", func() {
			Context("without a limit specified", func() {
				It("returns no builds", func() {
					returnedBuilds, _, err := job.BuildsWithTime(db.Page{})
					Expect(err).NotTo(HaveOccurred())

					Expect(returnedBuilds).To(BeEmpty())
				})
			})

			Context("when a limit specified", func() {
				It("returns a subset of the builds", func() {
					returnedBuilds, _, err := job.BuildsWithTime(db.Page{
						Limit: 2,
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(returnedBuilds).To(ConsistOf(builds[3], builds[2]))
				})
			})
		})

		Context("when providing boundaries", func() {
			Context("only to", func() {
				It("returns only those before to", func() {
					returnedBuilds, _, err := job.BuildsWithTime(db.Page{
						To:    int(builds[2].StartTime().Unix()),
						Limit: 50,
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(returnedBuilds).To(ConsistOf(builds[0], builds[1], builds[2]))
				})
			})

			Context("only from", func() {
				It("returns only those after from", func() {
					returnedBuilds, _, err := job.BuildsWithTime(db.Page{
						From:  int(builds[1].StartTime().Unix()),
						Limit: 50,
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(returnedBuilds).To(ConsistOf(builds[1], builds[2], builds[3]))
				})
			})

			Context("from and to", func() {
				It("returns only elements in the range", func() {
					returnedBuilds, _, err := job.BuildsWithTime(db.Page{
						From:  int(builds[1].StartTime().Unix()),
						To:    int(builds[2].StartTime().Unix()),
						Limit: 50,
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(returnedBuilds).To(ConsistOf(builds[1], builds[2]))
				})
			})
		})
	})

	Describe("Build", func() {
		var firstBuild db.Build

		Context("when a build exists", func() {
			BeforeEach(func() {
				var err error
				firstBuild, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())
			})

			It("finds the latest build", func() {
				secondBuild, err := job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				build, found, err := job.Build("latest")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.ID()).To(Equal(secondBuild.ID()))
				Expect(build.Status()).To(Equal(secondBuild.Status()))
			})

			It("finds the build", func() {
				build, found, err := job.Build(firstBuild.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.ID()).To(Equal(firstBuild.ID()))
				Expect(build.Status()).To(Equal(firstBuild.Status()))
			})
		})

		Context("when the build does not exist", func() {
			It("does not error", func() {
				build, found, err := job.Build("bogus-build")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(build).To(BeNil())
			})

			It("does not error finding the latest", func() {
				build, found, err := job.Build("latest")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(build).To(BeNil())
			})
		})

		Context("creating a build", func() {
			It("requests schedule on the job", func() {
				requestedSchedule := job.ScheduleRequestedTime()

				_, err := job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				found, err := job.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(job.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule))
			})
		})
	})

	Describe("RerunBuild", func() {
		var firstBuild db.Build
		var rerunErr error
		var rerunBuild db.Build
		var buildToRerun db.Build

		JustBeforeEach(func() {
			rerunBuild, rerunErr = job.RerunBuild(buildToRerun)
		})

		Context("when the first build exists", func() {
			BeforeEach(func() {
				var err error
				firstBuild, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				buildToRerun = firstBuild
			})

			It("finds the build", func() {
				Expect(rerunErr).ToNot(HaveOccurred())
				Expect(rerunBuild.Name()).To(Equal(fmt.Sprintf("%s.1", firstBuild.Name())))
				Expect(rerunBuild.RerunNumber()).To(Equal(1))

				build, found, err := job.Build(rerunBuild.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.ID()).To(Equal(rerunBuild.ID()))
				Expect(build.Status()).To(Equal(rerunBuild.Status()))
			})

			It("requests schedule on the job", func() {
				requestedSchedule := job.ScheduleRequestedTime()

				_, err := job.RerunBuild(buildToRerun)
				Expect(err).NotTo(HaveOccurred())

				found, err := job.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(job.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule))
			})

			Context("when there is an existing rerun build", func() {
				var rerun1 db.Build

				BeforeEach(func() {
					var err error
					rerun1, err = job.RerunBuild(buildToRerun)
					Expect(err).ToNot(HaveOccurred())
					Expect(rerun1.Name()).To(Equal(fmt.Sprintf("%s.1", firstBuild.Name())))
					Expect(rerun1.RerunNumber()).To(Equal(1))
				})

				It("increments the rerun build number", func() {
					Expect(rerunErr).ToNot(HaveOccurred())
					Expect(rerunBuild.Name()).To(Equal(fmt.Sprintf("%s.2", firstBuild.Name())))
					Expect(rerunBuild.RerunNumber()).To(Equal(rerun1.RerunNumber() + 1))
				})
			})

			Context("when we try to rerun a rerun build", func() {
				var rerun1 db.Build

				BeforeEach(func() {
					var err error
					rerun1, err = job.RerunBuild(buildToRerun)
					Expect(err).ToNot(HaveOccurred())
					Expect(rerun1.Name()).To(Equal(fmt.Sprintf("%s.1", firstBuild.Name())))
					Expect(rerun1.RerunNumber()).To(Equal(1))

					buildToRerun = rerun1
				})

				It("keeps the name of original build and increments the rerun build number", func() {
					Expect(rerunErr).ToNot(HaveOccurred())
					Expect(rerunBuild.Name()).To(Equal(fmt.Sprintf("%s.2", firstBuild.Name())))
					Expect(rerunBuild.RerunNumber()).To(Equal(rerun1.RerunNumber() + 1))
				})
			})
		})
	})

	Describe("ScheduleBuild", func() {
		var (
			schedulingBuild            db.Build
			scheduleFound, reloadFound bool
			schedulingErr              error
		)

		saveMaxInFlightPipeline := func() {
			BeforeEach(func() {
				var err error
				pipeline, _, err = team.SavePipeline("fake-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",

							Public: true,

							RawMaxInFlight: 2,

							PlanSequence: []atc.Step{
								{
									Config: &atc.PutStep{
										Name: "some-resource",
										Params: atc.Params{
											"some-param": "some-value",
										},
									},
								},
								{
									Config: &atc.GetStep{
										Name:     "some-input",
										Resource: "some-resource",
										Params: atc.Params{
											"some-param": "some-value",
										},
										Passed:  []string{"job-1", "job-2"},
										Trigger: true,
									},
								},
								{
									Config: &atc.TaskStep{
										Name:       "some-task",
										Privileged: true,
										ConfigPath: "some/config/path.yml",
										Config: &atc.TaskConfig{
											RootfsURI: "some-image",
										},
									},
								},
							},
						},
						{
							Name: "some-other-job",
						},
						{
							Name:   "some-private-job",
							Public: false,
						},
						{
							Name: "other-serial-group-job",
						},
						{
							Name: "different-serial-group-job",
						},
						{
							Name: "job-1",
						},
						{
							Name: "job-2",
						},
					},
					Resources: atc.ResourceConfigs{
						{
							Name: "some-resource",
							Type: "some-type",
						},
						{
							Name: "some-other-resource",
							Type: "some-type",
						},
					},
				}, pipeline.ConfigVersion(), false)
				Expect(err).ToNot(HaveOccurred())
			})
		}

		saveSerialGroupsPipeline := func() {
			BeforeEach(func() {
				var err error
				pipeline, _, err = team.SavePipeline("fake-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",

							Public: true,

							Serial: true,

							SerialGroups: []string{"serial-group"},

							RawMaxInFlight: 2,

							PlanSequence: []atc.Step{
								{
									Config: &atc.PutStep{
										Name: "some-resource",
										Params: atc.Params{
											"some-param": "some-value",
										},
									},
								},
								{
									Config: &atc.GetStep{
										Name:     "some-input",
										Resource: "some-resource",
										Params: atc.Params{
											"some-param": "some-value",
										},
										Passed:  []string{"job-1", "job-2"},
										Trigger: true,
									},
								},
								{
									Config: &atc.TaskStep{
										Name:       "some-task",
										Privileged: true,
										ConfigPath: "some/config/path.yml",
										Config: &atc.TaskConfig{
											RootfsURI: "some-image",
										},
									},
								},
							},
						},
						{
							Name: "some-other-job",
						},
						{
							Name:   "some-private-job",
							Public: false,
						},
						{
							Name:         "other-serial-group-job",
							SerialGroups: []string{"serial-group", "really-different-group"},
						},
						{
							Name:         "different-serial-group-job",
							SerialGroups: []string{"different-serial-group"},
						},
						{
							Name: "job-1",
						},
						{
							Name: "job-2",
						},
					},
					Resources: atc.ResourceConfigs{
						{
							Name: "some-resource",
							Type: "some-type",
						},
						{
							Name: "some-other-resource",
							Type: "some-type",
						},
					},
				}, pipeline.ConfigVersion(), false)
				Expect(err).ToNot(HaveOccurred())
			})
		}

		JustBeforeEach(func() {
			var found bool
			var err error
			job, found, err = pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			scheduleFound, schedulingErr = job.ScheduleBuild(schedulingBuild)

			reloadFound, err = schedulingBuild.Reload()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the scheduling build is created first", func() {
			BeforeEach(func() {
				var err error
				schedulingBuild, err = job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when the job config doesn't specify max in flight", func() {
				BeforeEach(func() {
					var created bool
					var err error
					pipeline, created, err = team.SavePipeline("other-pipeline", atc.Config{
						Jobs: atc.JobConfigs{
							{
								Name: "some-job",
							},
						},
					}, db.ConfigVersion(0), false)
					Expect(err).ToNot(HaveOccurred())
					Expect(created).To(BeTrue())

					var found bool
					job, found, err = pipeline.Job("some-job")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
				})

				It("schedules the build", func() {
					Expect(schedulingErr).ToNot(HaveOccurred())
					Expect(scheduleFound).To(BeTrue())
				})

				Context("when build exists", func() {
					Context("when the pipeline is paused", func() {
						BeforeEach(func() {
							err := pipeline.Pause()
							Expect(err).ToNot(HaveOccurred())
						})

						It("returns false", func() {
							Expect(schedulingErr).ToNot(HaveOccurred())
							Expect(scheduleFound).To(BeFalse())
							Expect(reloadFound).To(BeTrue())
							Expect(schedulingBuild.IsScheduled()).To(BeFalse())
						})
					})

					Context("when the job is paused", func() {
						BeforeEach(func() {
							err := job.Pause()
							Expect(err).ToNot(HaveOccurred())
						})

						It("returns false", func() {
							Expect(schedulingErr).ToNot(HaveOccurred())
							Expect(scheduleFound).To(BeFalse())
							Expect(reloadFound).To(BeTrue())
							Expect(schedulingBuild.IsScheduled()).To(BeFalse())
						})
					})

					Context("when the pipeline and job is not paused", func() {
						It("sets the build to scheduled", func() {
							Expect(schedulingErr).ToNot(HaveOccurred())
							Expect(scheduleFound).To(BeTrue())
							Expect(reloadFound).To(BeTrue())
							Expect(schedulingBuild.IsScheduled()).To(BeTrue())
						})
					})
				})

				Context("when the build does not exist", func() {
					var deleteFound bool
					BeforeEach(func() {
						var err error
						deleteFound, err = schedulingBuild.Delete()
						Expect(err).ToNot(HaveOccurred())
					})

					It("returns false", func() {
						Expect(schedulingErr).To(HaveOccurred())
						Expect(scheduleFound).To(BeFalse())
						Expect(reloadFound).To(BeFalse())
						Expect(deleteFound).To(BeTrue())
					})
				})
			})

			Context("when the job config specifies max in flight = 2", func() {
				Context("when there are 2 builds running", func() {
					var startedBuild, scheduledBuild db.Build

					BeforeEach(func() {
						var err error
						startedBuild, err = job.CreateBuild()
						Expect(err).ToNot(HaveOccurred())
						scheduled, err := job.ScheduleBuild(startedBuild)
						Expect(err).ToNot(HaveOccurred())
						Expect(scheduled).To(BeTrue())
						_, err = startedBuild.Start(atc.Plan{})
						Expect(err).NotTo(HaveOccurred())

						scheduledBuild, err = job.CreateBuild()
						Expect(err).NotTo(HaveOccurred())
						scheduled, err = job.ScheduleBuild(scheduledBuild)
						Expect(err).ToNot(HaveOccurred())
						Expect(scheduled).To(BeTrue())
						_, err = startedBuild.Start(atc.Plan{})
						Expect(err).NotTo(HaveOccurred())

						for _, s := range []db.BuildStatus{db.BuildStatusSucceeded, db.BuildStatusFailed, db.BuildStatusErrored, db.BuildStatusAborted} {
							finishedBuild, err := job.CreateBuild()
							Expect(err).NotTo(HaveOccurred())

							scheduled, err = job.ScheduleBuild(finishedBuild)
							Expect(err).NotTo(HaveOccurred())
							Expect(scheduled).To(BeTrue())

							err = finishedBuild.Finish(s)
							Expect(err).NotTo(HaveOccurred())
						}

						otherJob, found, err := pipeline.Job("some-other-job")
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())

						_, err = otherJob.CreateBuild()
						Expect(err).NotTo(HaveOccurred())
					})

					saveMaxInFlightPipeline()

					It("returns max in flight reached so it does not schedule", func() {
						Expect(schedulingErr).ToNot(HaveOccurred())
						Expect(scheduleFound).To(BeFalse())
						Expect(reloadFound).To(BeTrue())
					})
				})

				Context("when there is 1 build running", func() {
					BeforeEach(func() {
						startedBuild, err := job.CreateBuild()
						Expect(err).NotTo(HaveOccurred())
						scheduled, err := job.ScheduleBuild(startedBuild)
						Expect(err).NotTo(HaveOccurred())
						Expect(scheduled).To(BeTrue())
						_, err = startedBuild.Start(atc.Plan{})
						Expect(err).NotTo(HaveOccurred())

						for _, s := range []db.BuildStatus{db.BuildStatusSucceeded, db.BuildStatusFailed, db.BuildStatusErrored, db.BuildStatusAborted} {
							finishedBuild, err := job.CreateBuild()
							Expect(err).NotTo(HaveOccurred())

							scheduled, err = job.ScheduleBuild(finishedBuild)
							Expect(err).NotTo(HaveOccurred())
							Expect(scheduled).To(BeTrue())

							err = finishedBuild.Finish(s)
							Expect(err).NotTo(HaveOccurred())
						}

						err = job.SaveNextInputMapping(nil, true)
						Expect(err).NotTo(HaveOccurred())
					})

					saveMaxInFlightPipeline()

					It("schedules the build", func() {
						Expect(schedulingErr).ToNot(HaveOccurred())
						Expect(scheduleFound).To(BeTrue())
						Expect(reloadFound).To(BeTrue())
					})
				})
			})

			Context("when the job is in serial groups", func() {
				Context("when multiple jobs in the serial group is running", func() {
					BeforeEach(func() {
						var err error
						_, err = job.CreateBuild()
						Expect(err).NotTo(HaveOccurred())

						otherSerialJob, found, err := pipeline.Job("other-serial-group-job")
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())

						serialGroupBuild, err := otherSerialJob.CreateBuild()
						Expect(err).NotTo(HaveOccurred())

						scheduled, err := otherSerialJob.ScheduleBuild(serialGroupBuild)
						Expect(err).NotTo(HaveOccurred())
						Expect(scheduled).To(BeTrue())

						differentSerialJob, found, err := pipeline.Job("different-serial-group-job")
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())

						differentSerialGroupBuild, err := differentSerialJob.CreateBuild()
						Expect(err).NotTo(HaveOccurred())

						scheduled, err = differentSerialJob.ScheduleBuild(differentSerialGroupBuild)
						Expect(err).NotTo(HaveOccurred())
						Expect(scheduled).To(BeTrue())
					})

					saveSerialGroupsPipeline()

					It("does not schedule the build", func() {
						Expect(schedulingErr).ToNot(HaveOccurred())
						Expect(scheduleFound).To(BeFalse())
					})
				})

				Context("when no jobs in the serial groups are running", func() {
					BeforeEach(func() {
						otherSerialJob, found, err := pipeline.Job("other-serial-group-job")
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())

						serialGroupBuild, err := otherSerialJob.CreateBuild()
						Expect(err).NotTo(HaveOccurred())

						scheduled, err := otherSerialJob.ScheduleBuild(serialGroupBuild)
						Expect(err).NotTo(HaveOccurred())
						Expect(scheduled).To(BeTrue())

						err = serialGroupBuild.Finish(db.BuildStatusSucceeded)
						Expect(err).NotTo(HaveOccurred())

						differentSerialJob, found, err := pipeline.Job("different-serial-group-job")
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())

						differentSerialGroupBuild, err := differentSerialJob.CreateBuild()
						Expect(err).NotTo(HaveOccurred())

						scheduled, err = differentSerialJob.ScheduleBuild(differentSerialGroupBuild)
						Expect(err).NotTo(HaveOccurred())
						Expect(scheduled).To(BeTrue())

						err = job.SaveNextInputMapping(nil, true)
						Expect(err).NotTo(HaveOccurred())
					})

					saveSerialGroupsPipeline()

					It("does schedule the build", func() {
						Expect(schedulingErr).ToNot(HaveOccurred())
						Expect(scheduleFound).To(BeTrue())
						Expect(reloadFound).To(BeTrue())
					})
				})
			})
		})

		Context("when the scheduling build is not the first one created (with serial groups)", func() {
			Context("when the scheduling build has inputs determined as false", func() {
				BeforeEach(func() {
					var err error
					schedulingBuild, err = job.CreateBuild()
					Expect(err).NotTo(HaveOccurred())

					err = job.SaveNextInputMapping(nil, false)
					Expect(err).NotTo(HaveOccurred())
				})

				saveSerialGroupsPipeline()

				It("does not schedule because the inputs determined is false", func() {
					Expect(schedulingErr).ToNot(HaveOccurred())
					Expect(scheduleFound).To(BeFalse())
					Expect(reloadFound).To(BeTrue())
				})
			})

			Context("when another build within the serial group is scheduled first", func() {
				BeforeEach(func() {
					otherSerialJob, found, err := pipeline.Job("other-serial-group-job")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					_, err = otherSerialJob.CreateBuild()
					Expect(err).NotTo(HaveOccurred())

					err = otherSerialJob.SaveNextInputMapping(nil, true)
					Expect(err).NotTo(HaveOccurred())

					schedulingBuild, err = job.CreateBuild()
					Expect(err).NotTo(HaveOccurred())

					err = job.SaveNextInputMapping(nil, true)
					Expect(err).NotTo(HaveOccurred())
				})

				saveSerialGroupsPipeline()

				It("does not schedule because the build we are trying to schedule is not the next most pending build in the serial group", func() {
					Expect(schedulingErr).ToNot(HaveOccurred())
					Expect(scheduleFound).To(BeFalse())
					Expect(reloadFound).To(BeTrue())
				})
			})

			Context("when the scheduling build has it's inputs determined and created earlier", func() {
				BeforeEach(func() {
					var err error
					schedulingBuild, err = job.CreateBuild()
					Expect(err).NotTo(HaveOccurred())

					otherSerialJob, found, err := pipeline.Job("other-serial-group-job")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					_, err = otherSerialJob.CreateBuild()
					Expect(err).NotTo(HaveOccurred())

					err = job.SaveNextInputMapping(nil, true)
					Expect(err).NotTo(HaveOccurred())
					err = otherSerialJob.SaveNextInputMapping(nil, true)
					Expect(err).NotTo(HaveOccurred())
				})

				saveSerialGroupsPipeline()

				It("does schedule the build", func() {
					Expect(schedulingErr).ToNot(HaveOccurred())
					Expect(scheduleFound).To(BeTrue())
					Expect(reloadFound).To(BeTrue())
				})
			})

			Context("when the job is paused but has inputs determined", func() {
				BeforeEach(func() {
					var err error
					schedulingBuild, err = job.CreateBuild()
					Expect(err).NotTo(HaveOccurred())

					otherSerialJob, found, err := pipeline.Job("other-serial-group-job")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					_, err = otherSerialJob.CreateBuild()
					Expect(err).NotTo(HaveOccurred())

					err = job.SaveNextInputMapping(nil, true)
					Expect(err).NotTo(HaveOccurred())
					err = otherSerialJob.SaveNextInputMapping(nil, true)
					Expect(err).NotTo(HaveOccurred())

					err = job.Pause()
					Expect(err).NotTo(HaveOccurred())
				})

				saveSerialGroupsPipeline()

				It("does not schedule the build", func() {
					Expect(schedulingErr).ToNot(HaveOccurred())
					Expect(scheduleFound).To(BeFalse())
					Expect(reloadFound).To(BeTrue())
				})
			})

			Context("when there are other succeeded builds within the same serial group", func() {
				BeforeEach(func() {
					otherSerialJob, found, err := pipeline.Job("other-serial-group-job")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					succeededBuild, err := otherSerialJob.CreateBuild()
					Expect(err).NotTo(HaveOccurred())

					err = succeededBuild.Finish(db.BuildStatusSucceeded)
					Expect(err).NotTo(HaveOccurred())

					err = job.SaveNextInputMapping(nil, true)
					Expect(err).NotTo(HaveOccurred())
					err = otherSerialJob.SaveNextInputMapping(nil, true)
					Expect(err).NotTo(HaveOccurred())

					schedulingBuild, err = job.CreateBuild()
					Expect(err).NotTo(HaveOccurred())
				})

				saveSerialGroupsPipeline()

				It("does schedule builds because we only care about running or pending builds", func() {
					Expect(schedulingErr).ToNot(HaveOccurred())
					Expect(scheduleFound).To(BeTrue())
					Expect(reloadFound).To(BeTrue())
				})
			})

			Context("when the job we are trying to schedule has multiple serial groups", func() {
				BeforeEach(func() {
					otherSerialJob, found, err := pipeline.Job("some-job")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					_, err = otherSerialJob.CreateBuild()
					Expect(err).NotTo(HaveOccurred())

					job, found, err = pipeline.Job("other-serial-group-job")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					schedulingBuild, err = job.CreateBuild()
					Expect(err).NotTo(HaveOccurred())

					err = job.SaveNextInputMapping(nil, true)
					Expect(err).NotTo(HaveOccurred())
					err = otherSerialJob.SaveNextInputMapping(nil, true)
					Expect(err).NotTo(HaveOccurred())
				})

				saveSerialGroupsPipeline()

				It("does not schedule a build because the a build within one of the serial groups was created earlier", func() {
					Expect(schedulingErr).ToNot(HaveOccurred())
					Expect(scheduleFound).To(BeFalse())
					Expect(reloadFound).To(BeTrue())
				})
			})
		})
	})

	Describe("GetNextBuildInputs", func() {
		var (
			versions            []atc.ResourceVersion
			job                 db.Job
			resourceConfigScope db.ResourceConfigScope
			resource            db.Resource
			spanContext         db.SpanContext
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

			var created bool
			pipeline, created, err = team.SavePipeline("build-inputs-pipeline", atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name:     "some-input",
									Resource: "some-resource",
									Passed:   []string{"job-1", "job-2"},
									Trigger:  true,
								},
							},
							{
								Config: &atc.GetStep{
									Name:     "some-input-2",
									Resource: "some-resource",
									Passed:   []string{"job-1"},
									Trigger:  true,
								},
							},
							{
								Config: &atc.GetStep{
									Name:     "some-input-3",
									Resource: "some-resource",
									Trigger:  true,
								},
							},
						},
					},
					{
						Name: "job-1",
					},
					{
						Name: "job-2",
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-type",
					},
				},
			}, db.ConfigVersion(0), false)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).To(BeTrue())

			var found bool
			job, found, err = pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resource, found, err = pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resourceConfigScope, err = resource.SetResourceConfig(atc.Source{}, atc.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			spanContext = db.SpanContext{"fake": "version"}
			err = resourceConfigScope.SaveVersions(
				spanContext,
				[]atc.Version{
					{"version": "v1"},
					{"version": "v2"},
					{"version": "v3"},
				},
			)
			Expect(err).NotTo(HaveOccurred())

			reversions, _, found, err := resource.Versions(db.Page{Limit: 3}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			versions = []atc.ResourceVersion{reversions[2], reversions[1], reversions[0]}
		})

		Describe("partial next build inputs", func() {
			It("gets partial next build inputs for the given job name", func() {
				inputVersions := db.InputMapping{
					"some-input-2": db.InputResult{
						ResolveError: "disaster",
					},
				}

				err := job.SaveNextInputMapping(inputVersions, false)
				Expect(err).NotTo(HaveOccurred())

				buildInputs := []db.BuildInput{
					{
						Name:         "some-input-2",
						ResolveError: "disaster",
					},
				}

				actualBuildInputs, err := job.GetNextBuildInputs()
				Expect(err).NotTo(HaveOccurred())

				Expect(actualBuildInputs).To(ConsistOf(buildInputs))
			})

			It("gets full next build inputs for the given job name", func() {
				inputVersions := db.InputMapping{
					"some-input-1": db.InputResult{
						Input: &db.AlgorithmInput{
							AlgorithmVersion: db.AlgorithmVersion{
								Version:    db.ResourceVersion(convertToMD5(versions[0].Version)),
								ResourceID: resource.ID(),
							},
							FirstOccurrence: false,
						},
						PassedBuildIDs: []int{},
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
								Version:    db.ResourceVersion(convertToMD5(versions[2].Version)),
								ResourceID: resource.ID(),
							},
							FirstOccurrence: false,
						},
						PassedBuildIDs: []int{},
					},
				}

				err := job.SaveNextInputMapping(inputVersions, true)
				Expect(err).NotTo(HaveOccurred())

				buildInputs := []db.BuildInput{
					{
						Name:            "some-input-1",
						ResourceID:      resource.ID(),
						Version:         atc.Version{"version": "v1"},
						FirstOccurrence: false,
						Context:         spanContext,
					},
					{
						Name:            "some-input-2",
						ResourceID:      resource.ID(),
						Version:         atc.Version{"version": "v2"},
						FirstOccurrence: false,
						Context:         spanContext,
					},
					{
						Name:            "some-input-3",
						ResourceID:      resource.ID(),
						Version:         atc.Version{"version": "v3"},
						FirstOccurrence: false,
						Context:         spanContext,
					},
				}

				actualBuildInputs, err := job.GetNextBuildInputs()
				Expect(err).NotTo(HaveOccurred())

				Expect(actualBuildInputs).To(ConsistOf(buildInputs))
			})
		})
	})

	Describe("GetFullNextBuildInputs", func() {
		var (
			pipeline2           db.Pipeline
			versions            []atc.ResourceVersion
			job                 db.Job
			job2                db.Job
			resourceConfigScope db.ResourceConfigScope
			resource            db.Resource
			resource2           db.Resource
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
			job, found, err = pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resource, found, err = pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resourceConfigScope, err = resource.SetResourceConfig(atc.Source{}, atc.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			err = resourceConfigScope.SaveVersions(nil, []atc.Version{
				{"version": "v1"},
				{"version": "v2"},
				{"version": "v3"},
			})
			Expect(err).NotTo(HaveOccurred())

			// save metadata for v1
			_, err = resource.SaveUncheckedVersion(atc.Version{"version": "v1"}, db.ResourceConfigMetadataFields{
				db.ResourceConfigMetadataField{
					Name:  "name1",
					Value: "value1",
				},
			}, resourceConfigScope.ResourceConfig(), atc.VersionedResourceTypes{})
			Expect(err).NotTo(HaveOccurred())

			reversions, _, found, err := resource.Versions(db.Page{Limit: 3}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			versions = []atc.ResourceVersion{reversions[2], reversions[1], reversions[0]}

			config := atc.Config{
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
						Name: "some-resource",
						Type: "some-type",
					},
				},
			}

			pipeline2, _, err = team.SavePipeline("some-pipeline-2", config, 1, false)
			Expect(err).ToNot(HaveOccurred())

			resource2, found, err = pipeline2.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			job2, found, err = pipeline2.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		It("gets next build inputs for the given job name", func() {
			inputVersions := db.InputMapping{
				"some-input-1": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5(versions[0].Version)),
							ResourceID: resource.ID(),
						},
						FirstOccurrence: false,
					},
					PassedBuildIDs: []int{},
				},
				"some-input-2": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5(versions[1].Version)),
							ResourceID: resource.ID(),
						},
						FirstOccurrence: true,
					},
					PassedBuildIDs: []int{},
				},
			}
			err := job.SaveNextInputMapping(inputVersions, true)
			Expect(err).NotTo(HaveOccurred())

			pipeline2InputVersions := db.InputMapping{
				"some-input-3": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5(versions[2].Version)),
							ResourceID: resource2.ID(),
						},
						FirstOccurrence: false,
					},
					PassedBuildIDs: []int{},
				},
			}
			err = job2.SaveNextInputMapping(pipeline2InputVersions, true)
			Expect(err).NotTo(HaveOccurred())

			buildInputs := []db.BuildInput{
				{
					Name:            "some-input-1",
					ResourceID:      resource.ID(),
					Version:         atc.Version{"version": "v1"},
					FirstOccurrence: false,
				},
				{
					Name:            "some-input-2",
					ResourceID:      resource.ID(),
					Version:         atc.Version{"version": "v2"},
					FirstOccurrence: true,
				},
			}

			actualBuildInputs, found, err := job.GetFullNextBuildInputs()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(actualBuildInputs).To(ConsistOf(buildInputs))

			By("updating the set of next build inputs")
			inputVersions2 := db.InputMapping{
				"some-input-2": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5(versions[2].Version)),
							ResourceID: resource.ID(),
						},
						FirstOccurrence: false,
					},
					PassedBuildIDs: []int{},
				},
				"some-input-3": db.InputResult{
					Input: &db.AlgorithmInput{
						AlgorithmVersion: db.AlgorithmVersion{
							Version:    db.ResourceVersion(convertToMD5(versions[2].Version)),
							ResourceID: resource.ID(),
						},
						FirstOccurrence: true,
					},
					PassedBuildIDs: []int{},
				},
			}
			err = job.SaveNextInputMapping(inputVersions2, true)
			Expect(err).NotTo(HaveOccurred())

			buildInputs2 := []db.BuildInput{
				{
					Name:            "some-input-2",
					ResourceID:      resource.ID(),
					Version:         atc.Version{"version": "v3"},
					FirstOccurrence: false,
				},
				{
					Name:            "some-input-3",
					ResourceID:      resource.ID(),
					Version:         atc.Version{"version": "v3"},
					FirstOccurrence: true,
				},
			}

			actualBuildInputs2, found, err := job.GetFullNextBuildInputs()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(actualBuildInputs2).To(ConsistOf(buildInputs2))

			By("updating next build inputs to an empty set when the mapping is nil")
			err = job.SaveNextInputMapping(nil, true)
			Expect(err).NotTo(HaveOccurred())

			actualBuildInputs3, found, err := job.GetFullNextBuildInputs()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(actualBuildInputs3).To(BeEmpty())
		})

		It("distinguishes between a job with no inputs and a job with missing inputs", func() {
			By("initially returning not found")
			_, found, err := job.GetFullNextBuildInputs()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())

			By("returning found when an empty input mapping is saved")
			err = job.SaveNextInputMapping(db.InputMapping{}, true)
			Expect(err).NotTo(HaveOccurred())

			_, found, err = job.GetFullNextBuildInputs()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		It("does not grab inputs if inputs were not successfully determined", func() {
			inputVersions := db.InputMapping{
				"some-input-1": db.InputResult{
					ResolveError: "disaster",
				},
			}
			err := job.SaveNextInputMapping(inputVersions, false)
			Expect(err).NotTo(HaveOccurred())

			_, found, err := job.GetFullNextBuildInputs()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Describe("a build is created for a job", func() {
		var (
			build1DB      db.Build
			otherPipeline db.Pipeline
			otherJob      db.Job
		)

		BeforeEach(func() {
			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name: "some-other-resource",
						Type: "some-type",
					},
				},
			}
			var err error
			otherPipeline, _, err = team.SavePipeline("some-other-pipeline", pipelineConfig, db.ConfigVersion(1), false)
			Expect(err).ToNot(HaveOccurred())

			build1DB, err = job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			Expect(build1DB.ID()).NotTo(BeZero())
			Expect(build1DB.JobName()).To(Equal("some-job"))
			Expect(build1DB.Name()).To(Equal("1"))
			Expect(build1DB.Status()).To(Equal(db.BuildStatusPending))
			Expect(build1DB.IsScheduled()).To(BeFalse())

			var found bool
			otherJob, found, err = otherPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		It("becomes the next pending build for job", func() {
			nextPendings, err := job.GetPendingBuilds()
			Expect(err).NotTo(HaveOccurred())
			//time.Sleep(10 * time.Hour)
			Expect(nextPendings).NotTo(BeEmpty())
			Expect(nextPendings[0].ID()).To(Equal(build1DB.ID()))
		})

		Context("and another build for a different pipeline is created with the same job name", func() {
			BeforeEach(func() {
				otherBuild, err := otherJob.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				Expect(otherBuild.ID()).NotTo(BeZero())
				Expect(otherBuild.JobName()).To(Equal("some-job"))
				Expect(otherBuild.Name()).To(Equal("1"))
				Expect(otherBuild.Status()).To(Equal(db.BuildStatusPending))
				Expect(otherBuild.IsScheduled()).To(BeFalse())
			})

			It("does not change the next pending build for job", func() {
				nextPendingBuilds, err := job.GetPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(nextPendingBuilds).To(Equal([]db.Build{build1DB}))
			})
		})

		Context("when scheduled", func() {
			BeforeEach(func() {
				var err error
				var found bool
				found, err = job.ScheduleBuild(build1DB)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("remains the next pending build for job", func() {
				nextPendingBuilds, err := job.GetPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(nextPendingBuilds).NotTo(BeEmpty())
				Expect(nextPendingBuilds[0].ID()).To(Equal(build1DB.ID()))
			})
		})

		Context("when started", func() {
			BeforeEach(func() {
				started, err := build1DB.Start(atc.Plan{ID: "some-id"})
				Expect(err).NotTo(HaveOccurred())
				Expect(started).To(BeTrue())
			})

			It("saves the updated status, and the schema and private plan", func() {
				found, err := build1DB.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build1DB.Status()).To(Equal(db.BuildStatusStarted))
				Expect(build1DB.Schema()).To(Equal("exec.v2"))
				Expect(build1DB.PrivatePlan()).To(Equal(atc.Plan{ID: "some-id"}))
			})

			It("saves the build's start time", func() {
				found, err := build1DB.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build1DB.StartTime().Unix()).To(BeNumerically("~", time.Now().Unix(), 3))
			})
		})

		Context("when the build finishes", func() {
			BeforeEach(func() {
				err := build1DB.Finish(db.BuildStatusSucceeded)
				Expect(err).NotTo(HaveOccurred())
			})

			It("sets the build's status and end time", func() {
				found, err := build1DB.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build1DB.Status()).To(Equal(db.BuildStatusSucceeded))
				Expect(build1DB.EndTime().Unix()).To(BeNumerically("~", time.Now().Unix(), 3))
			})
		})

		Context("and another is created for the same job", func() {
			var build2DB db.Build

			BeforeEach(func() {
				var err error
				build2DB, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				Expect(build2DB.ID()).NotTo(BeZero())
				Expect(build2DB.ID()).NotTo(Equal(build1DB.ID()))
				Expect(build2DB.Name()).To(Equal("2"))
				Expect(build2DB.Status()).To(Equal(db.BuildStatusPending))
			})

			Describe("the first build", func() {
				It("remains the next pending build", func() {
					nextPendingBuilds, err := job.GetPendingBuilds()
					Expect(err).NotTo(HaveOccurred())
					Expect(nextPendingBuilds).To(HaveLen(2))
					Expect(nextPendingBuilds[0].ID()).To(Equal(build1DB.ID()))
					Expect(nextPendingBuilds[1].ID()).To(Equal(build2DB.ID()))
				})
			})
		})

		Context("when there is a rerun build created for an old build", func() {
			var rerunBuild db.Build
			var newBuild db.Build
			var newerBuild db.Build

			BeforeEach(func() {
				var err error
				newBuild, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				newerBuild, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				err = newBuild.Finish(db.BuildStatusSucceeded)
				Expect(err).NotTo(HaveOccurred())

				rerunBuild, err = job.RerunBuild(newBuild)
				Expect(err).NotTo(HaveOccurred())

				Expect(rerunBuild.ID()).NotTo(BeZero())
				Expect(rerunBuild.ID()).NotTo(Equal(newBuild.ID()))
				Expect(rerunBuild.Name()).To(Equal("2.1"))
				Expect(rerunBuild.Status()).To(Equal(db.BuildStatusPending))
			})

			It("orders the builds with regular build first and then rerun of old build", func() {
				nextPendingBuilds, err := job.GetPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(nextPendingBuilds)).To(Equal(3))
				Expect(nextPendingBuilds[0].Name()).To(Equal(build1DB.Name()))
				Expect(nextPendingBuilds[1].Name()).To(Equal(rerunBuild.Name()))
				Expect(nextPendingBuilds[2].Name()).To(Equal(newerBuild.Name()))
			})
		})

		Context("when there is a rerun build created for the newest build", func() {
			var rerunBuild db.Build
			var newBuild db.Build
			var newerBuild db.Build

			BeforeEach(func() {
				var err error
				newBuild, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				rerunBuild, err = job.RerunBuild(newBuild)
				Expect(err).NotTo(HaveOccurred())

				newerBuild, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				Expect(rerunBuild.ID()).NotTo(BeZero())
				Expect(rerunBuild.ID()).NotTo(Equal(newBuild.ID()))
				Expect(rerunBuild.Name()).To(Equal("2.1"))
				Expect(rerunBuild.Status()).To(Equal(db.BuildStatusPending))
			})

			It("orders the builds with rerun of new build", func() {
				nextPendingBuilds, err := job.GetPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(nextPendingBuilds)).To(Equal(4))
				Expect(nextPendingBuilds[0].ID()).To(Equal(build1DB.ID()))
				Expect(nextPendingBuilds[1].ID()).To(Equal(newBuild.ID()))
				Expect(nextPendingBuilds[2].ID()).To(Equal(rerunBuild.ID()))
				Expect(nextPendingBuilds[3].ID()).To(Equal(newerBuild.ID()))
			})
		})

		Context("when there are multiple reruns for multiple pending builds", func() {
			var rerunBuild db.Build
			var rerunBuild2 db.Build
			var rerunBuild3 db.Build
			var newBuild db.Build
			var newerBuild db.Build

			BeforeEach(func() {
				var err error
				newBuild, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				newerBuild, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				rerunBuild3, err = job.RerunBuild(newerBuild)
				Expect(err).NotTo(HaveOccurred())

				rerunBuild, err = job.RerunBuild(newBuild)
				Expect(err).NotTo(HaveOccurred())

				rerunBuild2, err = job.RerunBuild(rerunBuild)
				Expect(err).NotTo(HaveOccurred())

				Expect(rerunBuild.ID()).NotTo(BeZero())
				Expect(rerunBuild.ID()).NotTo(Equal(newBuild.ID()))
				Expect(rerunBuild.Name()).To(Equal("2.1"))
				Expect(rerunBuild.Status()).To(Equal(db.BuildStatusPending))
			})

			It("orders the builds with ascending reruns following original builds", func() {
				nextPendingBuilds, err := job.GetPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(nextPendingBuilds)).To(Equal(6))
				Expect(nextPendingBuilds[0].Name()).To(Equal(build1DB.Name()))
				Expect(nextPendingBuilds[1].Name()).To(Equal(newBuild.Name()))
				Expect(nextPendingBuilds[2].Name()).To(Equal(rerunBuild.Name()))
				Expect(nextPendingBuilds[3].Name()).To(Equal(rerunBuild2.Name()))
				Expect(nextPendingBuilds[4].Name()).To(Equal(newerBuild.Name()))
				Expect(nextPendingBuilds[5].Name()).To(Equal(rerunBuild3.Name()))
			})
		})
	})

	Describe("EnsurePendingBuildExists", func() {
		Context("when only a started build exists", func() {
			It("creates a build and updates the next build for the job", func() {
				err := job.EnsurePendingBuildExists(context.TODO())
				Expect(err).NotTo(HaveOccurred())

				pendingBuilds, err := job.GetPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(pendingBuilds).To(HaveLen(1))

				_, nextBuild, err := job.FinishedAndNextBuild()
				Expect(err).NotTo(HaveOccurred())
				Expect(pendingBuilds[0].ID()).To(Equal(nextBuild.ID()))
			})

			Context("when tracing is configured", func() {
				BeforeEach(func() {
					tracing.ConfigureTraceProvider(&tracing.TestTraceProvider{})
				})

				AfterEach(func() {
					tracing.Configured = false
				})

				It("propagates span context", func() {
					ctx, span := tracing.StartSpan(context.Background(), "fake-operation", nil)
					traceID := span.SpanContext().TraceIDString()

					job.EnsurePendingBuildExists(ctx)

					pendingBuilds, _ := job.GetPendingBuilds()
					spanContext := pendingBuilds[0].SpanContext()
					traceParent := spanContext.Get(propagators.TraceparentHeader)
					Expect(traceParent).To(ContainSubstring(traceID))
				})
			})

			It("doesn't create another build the second time it's called", func() {
				err := job.EnsurePendingBuildExists(context.TODO())
				Expect(err).NotTo(HaveOccurred())

				err = job.EnsurePendingBuildExists(context.TODO())
				Expect(err).NotTo(HaveOccurred())

				builds2, err := job.GetPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(builds2).To(HaveLen(1))

				started, err := builds2[0].Start(atc.Plan{})
				Expect(err).NotTo(HaveOccurred())
				Expect(started).To(BeTrue())

				builds2, err = job.GetPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(builds2).To(HaveLen(0))
			})
		})
	})

	Describe("Clear task cache", func() {
		Context("when task cache exists", func() {
			var (
				someOtherJob db.Job
				rowsDeleted  int64
			)

			BeforeEach(func() {
				var (
					err   error
					found bool
				)

				usedTaskCache, err := taskCacheFactory.FindOrCreate(job.ID(), "some-task", "some-path")
				Expect(err).ToNot(HaveOccurred())

				_, err = workerTaskCacheFactory.FindOrCreate(db.WorkerTaskCache{
					TaskCache:  usedTaskCache,
					WorkerName: defaultWorker.Name(),
				})
				Expect(err).ToNot(HaveOccurred())

				someOtherJob, found, err = pipeline.Job("some-other-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(someOtherJob).ToNot(BeNil())

				otherUsedTaskCache, err := taskCacheFactory.FindOrCreate(someOtherJob.ID(), "some-other-task", "some-other-path")
				Expect(err).ToNot(HaveOccurred())

				_, err = workerTaskCacheFactory.FindOrCreate(db.WorkerTaskCache{
					TaskCache:  otherUsedTaskCache,
					WorkerName: defaultWorker.Name(),
				})
				Expect(err).ToNot(HaveOccurred())

			})

			Context("when a path is provided", func() {
				BeforeEach(func() {
					var err error
					rowsDeleted, err = job.ClearTaskCache("some-task", "some-path")
					Expect(err).NotTo(HaveOccurred())
				})

				It("deletes a row from the task_caches table", func() {
					Expect(rowsDeleted).To(Equal(int64(1)))
				})

				It("removes the task cache", func() {
					usedTaskCache, found, err := taskCacheFactory.Find(job.ID(), "some-task", "some-path")
					Expect(err).ToNot(HaveOccurred())
					Expect(usedTaskCache).To(BeNil())
					Expect(found).To(BeFalse())
				})

				It("doesn't remove other jobs caches", func() {
					otherUsedTaskCache, found, err := taskCacheFactory.Find(someOtherJob.ID(), "some-other-task", "some-other-path")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())

					_, err = workerTaskCacheFactory.FindOrCreate(db.WorkerTaskCache{
						TaskCache:  otherUsedTaskCache,
						WorkerName: defaultWorker.Name(),
					})
					Expect(err).ToNot(HaveOccurred())
				})

				Context("but the cache path doesn't exist", func() {
					BeforeEach(func() {
						var err error
						rowsDeleted, err = job.ClearTaskCache("some-task", "some-nonexistent-path")
						Expect(err).NotTo(HaveOccurred())

					})
					It("deletes 0 rows", func() {
						Expect(rowsDeleted).To(Equal(int64(0)))
					})
				})
			})

			Context("when a path is not provided", func() {
				Context("when a non-existent step-name is provided", func() {
					BeforeEach(func() {
						var err error
						rowsDeleted, err = job.ClearTaskCache("some-nonexistent-task", "")
						Expect(err).NotTo(HaveOccurred())
					})

					It("does not delete any rows from the task_caches table", func() {
						Expect(rowsDeleted).To(BeZero())
					})

					It("should not delete any task steps", func() {
						usedTaskCache, found, err := taskCacheFactory.Find(job.ID(), "some-task", "some-path")
						Expect(err).ToNot(HaveOccurred())
						Expect(found).To(BeTrue())
						Expect(err).ToNot(HaveOccurred())

						_, found, err = workerTaskCacheFactory.Find(db.WorkerTaskCache{
							TaskCache:  usedTaskCache,
							WorkerName: defaultWorker.Name(),
						})
						Expect(found).To(BeTrue())
						Expect(err).ToNot(HaveOccurred())
					})

				})

				Context("when an existing step-name is provided", func() {
					BeforeEach(func() {
						var err error
						rowsDeleted, err = job.ClearTaskCache("some-task", "")
						Expect(err).NotTo(HaveOccurred())
					})

					It("deletes a row from the task_caches table", func() {
						Expect(rowsDeleted).To(Equal(int64(1)))
					})

					It("removes the task cache", func() {
						_, found, err := taskCacheFactory.Find(job.ID(), "some-task", "some-path")
						Expect(found).To(BeFalse())
						Expect(err).ToNot(HaveOccurred())
					})

					It("doesn't remove other jobs caches", func() {
						_, found, err := taskCacheFactory.Find(someOtherJob.ID(), "some-other-task", "some-other-path")
						Expect(found).To(BeTrue())
						Expect(err).ToNot(HaveOccurred())
					})
				})
			})
		})
	})

	Describe("New Inputs", func() {
		It("starts out as false", func() {
			Expect(job.HasNewInputs()).To(BeFalse())
		})

		It("can be set to true then back to false", func() {
			job.SetHasNewInputs(true)

			found, err := job.Reload()

			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(job.HasNewInputs()).To(BeTrue())

			job.SetHasNewInputs(false)

			found, err = job.Reload()

			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(job.HasNewInputs()).To(BeFalse())
		})
	})

	Describe("AlgorithmInputs", func() {
		var inputsJob db.Job
		var inputsPipeline db.Pipeline
		var inputs db.InputConfigs

		JustBeforeEach(func() {
			var err error
			inputs, err = inputsJob.AlgorithmInputs()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when there is an input configured for the job", func() {
			BeforeEach(func() {
				var err error
				inputsPipeline, _, err = team.SavePipeline("inputs-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
							PlanSequence: []atc.Step{
								{
									Config: &atc.GetStep{
										Name:     "some-input",
										Resource: "some-resource",
										Params: atc.Params{
											"some-param": "some-value",
										},
										Passed:  []string{"job-1", "job-2"},
										Trigger: true,
										Version: &atc.VersionConfig{Every: true},
									},
								},
							},
						},
						{
							Name: "job-1",
						},
						{
							Name: "job-2",
						},
					},
					Resources: atc.ResourceConfigs{
						{
							Name: "some-resource",
							Type: "some-type",
						},
					},
				}, db.ConfigVersion(0), false)
				Expect(err).ToNot(HaveOccurred())

				var found bool
				inputsJob, found, err = inputsPipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("returns the input for the job", func() {
				job1, found, err := inputsPipeline.Job("job-1")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				job2, found, err := inputsPipeline.Job("job-2")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				someResource, found, err := inputsPipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(inputs).To(Equal(db.InputConfigs{
					{
						Name:       "some-input",
						JobID:      inputsJob.ID(),
						ResourceID: someResource.ID(),
						Passed: db.JobSet{
							job1.ID(): true,
							job2.ID(): true,
						},
						UseEveryVersion: true,
						Trigger:         true,
					},
				}))
			})
		})

		Context("when the input is pinned through the get step", func() {
			BeforeEach(func() {
				var err error
				inputsPipeline, _, err = team.SavePipeline("inputs-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
							PlanSequence: []atc.Step{
								{
									Config: &atc.GetStep{
										Name:     "some-pinned-input",
										Resource: "some-resource",
										Version:  &atc.VersionConfig{Pinned: atc.Version{"input": "pinned"}},
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
					},
				}, db.ConfigVersion(0), false)
				Expect(err).ToNot(HaveOccurred())

				var found bool
				inputsJob, found, err = inputsPipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("pins the inputs to that version", func() {
				someResource, found, err := inputsPipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(inputs).To(Equal(db.InputConfigs{
					{
						Name:          "some-pinned-input",
						JobID:         inputsJob.ID(),
						ResourceID:    someResource.ID(),
						PinnedVersion: atc.Version{"input": "pinned"},
					},
				}))
			})

			Context("when the input is also pinned through the api", func() {
				BeforeEach(func() {
					pinnedResource, found, err := inputsPipeline.Resource("some-resource")
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

					resourceConfigScope, err := pinnedResource.SetResourceConfig(atc.Source{"some": "source"}, atc.VersionedResourceTypes{})
					Expect(err).ToNot(HaveOccurred())

					err = resourceConfigScope.SaveVersions(nil, []atc.Version{
						{"api": "pinned"},
					})
					Expect(err).NotTo(HaveOccurred())

					versionID, found, err := pinnedResource.ResourceConfigVersionID(atc.Version{"api": "pinned"})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					pinned, err := pinnedResource.PinVersion(versionID)
					Expect(err).NotTo(HaveOccurred())
					Expect(pinned).To(BeTrue())
				})

				It("resolves the pinned version to the version pinned through the get step", func() {
					someResource, found, err := inputsPipeline.Resource("some-resource")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					Expect(inputs).To(Equal(db.InputConfigs{
						{
							Name:          "some-pinned-input",
							JobID:         inputsJob.ID(),
							ResourceID:    someResource.ID(),
							PinnedVersion: atc.Version{"input": "pinned"},
						},
					}))
				})
			})
		})

		Context("when the input is pinned through the resource config", func() {
			BeforeEach(func() {
				var err error
				inputsPipeline, _, err = team.SavePipeline("inputs-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
							PlanSequence: []atc.Step{
								{
									Config: &atc.GetStep{
										Name:     "some-pinned-input",
										Resource: "some-resource",
									},
								},
							},
						},
					},
					Resources: atc.ResourceConfigs{
						{
							Name:    "some-resource",
							Type:    "some-type",
							Source:  atc.Source{"some": "source"},
							Version: atc.Version{"some": "version"},
						},
					},
				}, db.ConfigVersion(0), false)
				Expect(err).ToNot(HaveOccurred())

				var found bool
				inputsJob, found, err = inputsPipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("pins the inputs to that version", func() {
				someResource, found, err := inputsPipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(inputs).To(Equal(db.InputConfigs{
					{
						Name:          "some-pinned-input",
						JobID:         inputsJob.ID(),
						ResourceID:    someResource.ID(),
						PinnedVersion: atc.Version{"some": "version"},
					},
				}))
			})
		})

		Context("when the input is pinned through the api", func() {
			BeforeEach(func() {
				var err error
				inputsPipeline, _, err = team.SavePipeline("inputs-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
							PlanSequence: []atc.Step{
								{
									Config: &atc.GetStep{
										Name:     "some-pinned-input",
										Resource: "some-resource",
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
					},
				}, db.ConfigVersion(0), false)
				Expect(err).ToNot(HaveOccurred())

				var found bool
				inputsJob, found, err = inputsPipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				pinnedResource, found, err := inputsPipeline.Resource("some-resource")
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

				resourceConfigScope, err := pinnedResource.SetResourceConfig(atc.Source{"some": "source"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				err = resourceConfigScope.SaveVersions(nil, []atc.Version{
					{"some": "version"},
				})
				Expect(err).NotTo(HaveOccurred())

				versionID, found, err := pinnedResource.ResourceConfigVersionID(atc.Version{"some": "version"})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				pinned, err := pinnedResource.PinVersion(versionID)
				Expect(err).NotTo(HaveOccurred())
				Expect(pinned).To(BeTrue())
			})

			It("pins the inputs to that version", func() {
				someResource, found, err := inputsPipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(inputs).To(Equal(db.InputConfigs{
					{
						Name:          "some-pinned-input",
						JobID:         inputsJob.ID(),
						ResourceID:    someResource.ID(),
						PinnedVersion: atc.Version{"some": "version"},
					},
				}))
			})
		})

		Context("when there are multiple inputs", func() {
			BeforeEach(func() {
				var err error
				inputsPipeline, _, err = team.SavePipeline("inputs-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
							PlanSequence: []atc.Step{
								{
									Config: &atc.GetStep{
										Name:     "some-input",
										Resource: "some-resource",
										Trigger:  true,
										Version:  &atc.VersionConfig{Every: true},
									},
								},
								{
									Config: &atc.GetStep{
										Name: "some-resource",
									},
								},
								{
									Config: &atc.GetStep{
										Name:    "some-other-resource",
										Trigger: true,
										Version: &atc.VersionConfig{Latest: true},
									},
								},
							},
						},
						{
							Name: "some-other-job",
							PlanSequence: []atc.Step{
								{
									Config: &atc.GetStep{
										Name:     "other-job-resource",
										Resource: "some-resource",
									},
								},
							},
						},
					},
					Resources: atc.ResourceConfigs{
						{
							Name: "some-resource",
							Type: "some-type",
						},
						{
							Name: "some-other-resource",
							Type: "some-type",
						},
					},
				}, db.ConfigVersion(0), false)
				Expect(err).ToNot(HaveOccurred())

				var found bool
				inputsJob, found, err = inputsPipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("returns all the inputs correctly", func() {
				someResource, found, err := inputsPipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				someOtherResource, found, err := inputsPipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(inputs).To(HaveLen(3))
				Expect(inputs).To(ConsistOf(
					db.InputConfig{
						Name:            "some-input",
						JobID:           inputsJob.ID(),
						ResourceID:      someResource.ID(),
						UseEveryVersion: true,
						Trigger:         true,
					},
					db.InputConfig{
						Name:       "some-resource",
						JobID:      inputsJob.ID(),
						ResourceID: someResource.ID(),
					},
					db.InputConfig{
						Name:       "some-other-resource",
						JobID:      inputsJob.ID(),
						ResourceID: someOtherResource.ID(),
						Trigger:    true,
					}))
			})
		})

		Context("when the job has puts and tasks", func() {
			BeforeEach(func() {
				var err error
				inputsPipeline, _, err = team.SavePipeline("inputs-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
							PlanSequence: []atc.Step{
								{
									Config: &atc.PutStep{
										Name: "some-resource",
									},
								},
								{
									Config: &atc.TaskStep{
										Name:       "some-task",
										Privileged: true,
										ConfigPath: "some/config/path.yml",
										Config: &atc.TaskConfig{
											RootfsURI: "some-image",
										},
									},
								},
								{
									Config: &atc.GetStep{
										Name: "some-resource",
									},
								},
							},
						},
					},
					Resources: atc.ResourceConfigs{
						{
							Name: "some-resource",
							Type: "some-type",
						},
					},
				}, db.ConfigVersion(0), false)
				Expect(err).ToNot(HaveOccurred())

				var found bool
				inputsJob, found, err = inputsPipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("only returns the gets (inputs to the job)", func() {
				someResource, found, err := inputsPipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(inputs).To(Equal(db.InputConfigs{
					{
						Name:       "some-resource",
						JobID:      inputsJob.ID(),
						ResourceID: someResource.ID(),
					},
				}))
			})
		})
	})

	Describe("Inputs", func() {
		var inputsJob db.Job

		BeforeEach(func() {
			inputsPipeline, _, err := team.SavePipeline("inputs-pipeline", atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.PutStep{
									Name: "some-resource",
								},
							},
							{
								Config: &atc.GetStep{
									Name:     "some-input",
									Resource: "some-resource",
									Params: atc.Params{
										"some-param": "some-value",
									},
									Passed:  []string{"job-1", "job-2"},
									Trigger: true,
									Version: &atc.VersionConfig{Every: true},
								},
							},
							{
								Config: &atc.TaskStep{
									Name:       "some-task",
									Privileged: true,
									ConfigPath: "some/config/path.yml",
									Config: &atc.TaskConfig{
										RootfsURI: "some-image",
									},
								},
							},
							{
								Config: &atc.GetStep{
									Name: "some-resource",
								},
							},
							{
								Config: &atc.GetStep{
									Name:     "some-other-input",
									Resource: "some-resource",
									Version:  &atc.VersionConfig{Latest: true},
								},
							},
							{
								Config: &atc.GetStep{
									Name:    "some-other-resource",
									Trigger: true,
									Version: &atc.VersionConfig{Pinned: atc.Version{"pinned": "version"}},
								},
							},
						},
					},
					{
						Name: "some-other-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name:     "other-job-resource",
									Resource: "some-resource",
								},
							},
						},
					},
					{
						Name: "job-1",
					},
					{
						Name: "job-2",
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-type",
					},
					{
						Name: "some-other-resource",
						Type: "some-type",
					},
				},
			}, db.ConfigVersion(0), false)
			Expect(err).ToNot(HaveOccurred())

			var found bool
			inputsJob, found, err = inputsPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		It("returns inputs for the job", func() {
			inputs, err := inputsJob.Inputs()
			Expect(err).ToNot(HaveOccurred())

			Expect(inputs).To(Equal([]atc.JobInput{
				{
					Name:     "some-input",
					Resource: "some-resource",
					Passed:   []string{"job-1", "job-2"},
					Trigger:  true,
					Version:  &atc.VersionConfig{Every: true},
				},
				{
					Name:     "some-other-input",
					Resource: "some-resource",
					Version:  &atc.VersionConfig{Latest: true},
				},
				{
					Name:     "some-other-resource",
					Resource: "some-other-resource",
					Trigger:  true,
					Version:  &atc.VersionConfig{Pinned: atc.Version{"pinned": "version"}},
				},
				{
					Name:     "some-resource",
					Resource: "some-resource",
				},
			}))
		})
	})

	Describe("Outputs", func() {
		var outputsJob db.Job

		BeforeEach(func() {
			outputsPipeline, _, err := team.SavePipeline("outputs-pipeline", atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.PutStep{
									Name: "some-other-resource",
								},
							},
							{
								Config: &atc.TaskStep{
									Name:       "some-task",
									Privileged: true,
									ConfigPath: "some/config/path.yml",
									Config: &atc.TaskConfig{
										RootfsURI: "some-image",
									},
								},
							},
							{
								Config: &atc.GetStep{
									Name: "some-resource",
								},
							},
							{
								Config: &atc.PutStep{
									Name:     "some-output",
									Resource: "some-resource",
								},
							},
							{
								Config: &atc.PutStep{
									Name:     "some-other-output",
									Resource: "some-resource",
								},
							},
						},
					},
					{
						Name: "some-other-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.PutStep{
									Name:     "other-job-resource",
									Resource: "some-resource",
								},
							},
						},
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-type",
					},
					{
						Name: "some-other-resource",
						Type: "some-type",
					},
				},
			}, db.ConfigVersion(0), false)
			Expect(err).ToNot(HaveOccurred())

			var found bool
			outputsJob, found, err = outputsPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		It("returns outputs for the job", func() {
			outputs, err := outputsJob.Outputs()
			Expect(err).ToNot(HaveOccurred())

			Expect(outputs).To(Equal([]atc.JobOutput{
				{
					Name:     "some-other-output",
					Resource: "some-resource",
				},
				{
					Name:     "some-other-resource",
					Resource: "some-other-resource",
				},
				{
					Name:     "some-output",
					Resource: "some-resource",
				},
			}))
		})
	})
})
