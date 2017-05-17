package db_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/dbng"
	"github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PipelineDB", func() {
	var dbConn db.Conn
	var listener *pq.Listener

	var pipelineDBFactory db.PipelineDBFactory
	var sqlDB *db.SQLDB
	var teamDBFactory db.TeamDBFactory
	var teamFactory dbng.TeamFactory

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.OpenDB())
		dbngConn := postgresRunner.OpenConn()

		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		lockFactory := lock.NewLockFactory(postgresRunner.OpenSingleton())
		sqlDB = db.NewSQL(dbConn, bus, lockFactory)
		pipelineDBFactory = db.NewPipelineDBFactory(dbConn, bus, lockFactory)
		teamDBFactory = db.NewTeamDBFactory(dbConn, bus, lockFactory)
		teamFactory = dbng.NewTeamFactory(dbngConn, lockFactory, dbng.NewNoEncryption())
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	pipelineConfig := atc.Config{
		Groups: atc.GroupConfigs{
			{
				Name:      "some-group",
				Jobs:      []string{"job-1", "job-2"},
				Resources: []string{"some-resource", "some-other-resource"},
			},
		},

		Resources: atc.ResourceConfigs{
			{
				Name: "some-resource",
				Type: "some-type",
				Source: atc.Source{
					"source-config": "some-value",
				},
			},
			{
				Name: "some-other-resource",
				Type: "some-type",
				Source: atc.Source{
					"source-config": "some-value",
				},
			},
			{
				Name: "some-really-other-resource",
				Type: "some-type",
				Source: atc.Source{
					"source-config": "some-value",
				},
			},
		},

		ResourceTypes: atc.ResourceTypes{
			{
				Name: "some-resource-type",
				Type: "some-type",
				Source: atc.Source{
					"source-config": "some-value",
				},
			},
		},

		Jobs: atc.JobConfigs{
			{
				Name: "some-job",

				Public: true,

				Serial: true,

				SerialGroups: []string{"serial-group"},

				Plan: atc.PlanSequence{
					{
						Put: "some-resource",
						Params: atc.Params{
							"some-param": "some-value",
						},
					},
					{
						Get:      "some-input",
						Resource: "some-resource",
						Params: atc.Params{
							"some-param": "some-value",
						},
						Passed:  []string{"job-1", "job-2"},
						Trigger: true,
					},
					{
						Task:           "some-task",
						Privileged:     true,
						TaskConfigPath: "some/config/path.yml",
						TaskConfig: &atc.TaskConfig{
							RootFsUri: "some-image",
						},
					},
				},
			},
			{
				Name:   "some-other-job",
				Serial: true,
			},
			{
				Name: "a-job",
			},
			{
				Name: "shared-job",
			},
			{
				Name: "random-job",
			},
			{
				Name:         "other-serial-group-job",
				SerialGroups: []string{"serial-group", "really-different-group"},
			},
			{
				Name:         "different-serial-group-job",
				SerialGroups: []string{"different-serial-group"},
			},
		},
	}

	otherPipelineConfig := atc.Config{
		Groups: atc.GroupConfigs{
			{
				Name:      "some-group",
				Jobs:      []string{"job-1", "job-2"},
				Resources: []string{"some-resource", "some-other-resource"},
			},
		},

		Resources: atc.ResourceConfigs{
			{
				Name: "some-resource",
				Type: "some-type",
				Source: atc.Source{
					"source-config": "some-value",
				},
			},
			{
				Name: "some-other-resource",
				Type: "some-type",
				Source: atc.Source{
					"source-config": "some-value",
				},
			},
		},

		Jobs: atc.JobConfigs{
			{
				Name: "some-job",
			},
			{
				Name: "some-other-job",
			},
			{
				Name: "a-job",
			},
			{
				Name: "shared-job",
			},
			{
				Name: "other-serial-group-job",
			},
		},
	}

	var (
		teamDB             db.TeamDB
		pipelineDB         db.PipelineDB
		otherPipelineDB    db.PipelineDB
		savedPipeline      db.SavedPipeline
		otherSavedPipeline db.SavedPipeline
		savedTeam          db.SavedTeam
	)

	BeforeEach(func() {
		var err error
		savedTeam, err = sqlDB.CreateTeam(db.Team{Name: "some-team"})
		Expect(err).NotTo(HaveOccurred())

		teamDB = teamDBFactory.GetTeamDB("some-team")

		savedPipeline, _, err = teamDB.SaveConfigToBeDeprecated("a-pipeline-name", pipelineConfig, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		otherSavedPipeline, _, err = teamDB.SaveConfigToBeDeprecated("other-pipeline-name", otherPipelineConfig, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDB = pipelineDBFactory.Build(savedPipeline)
		otherPipelineDB = pipelineDBFactory.Build(otherSavedPipeline)
	})

	Describe("ScopedName", func() {
		It("concatenates the pipeline name with the passed in name", func() {
			pipelineDB := pipelineDBFactory.Build(db.SavedPipeline{
				Pipeline: db.Pipeline{
					Name: "some-pipeline",
				},
			})
			Expect(pipelineDB.ScopedName("something-else")).To(Equal("some-pipeline:something-else"))
		})
	})

	Describe("Reload", func() {
		It("can manage multiple pipeline configurations", func() {
			By("returning the saved config to later gets")
			Expect(pipelineDB.Config()).To(Equal(pipelineConfig))
			Expect(pipelineDB.ConfigVersion()).NotTo(Equal(db.ConfigVersion(0)))

			Expect(otherPipelineDB.Config()).To(Equal(otherPipelineConfig))
			Expect(otherPipelineDB.ConfigVersion()).NotTo(Equal(db.ConfigVersion(0)))

			updatedConfig := pipelineConfig

			updatedConfig.Groups = append(pipelineConfig.Groups, atc.GroupConfig{
				Name: "new-group",
				Jobs: []string{"new-job-1", "new-job-2"},
			})

			updatedConfig.Resources = append(pipelineConfig.Resources, atc.ResourceConfig{
				Name: "new-resource",
				Type: "new-type",
				Source: atc.Source{
					"new-source-config": "new-value",
				},
			})

			updatedConfig.Jobs = append(pipelineConfig.Jobs, atc.JobConfig{
				Name: "new-job",
				Plan: atc.PlanSequence{
					{
						Get:      "new-input",
						Resource: "new-resource",
						Params: atc.Params{
							"new-param": "new-value",
						},
					},
					{
						Task:           "some-task",
						TaskConfigPath: "new/config/path.yml",
					},
				},
			})

			By("being able to update the config with a valid config")
			_, _, err := teamDB.SaveConfigToBeDeprecated("a-pipeline-name", updatedConfig, pipelineDB.ConfigVersion(), db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = teamDB.SaveConfigToBeDeprecated("other-pipeline-name", updatedConfig, otherPipelineDB.ConfigVersion(), db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			By("returning the updated config")
			found, err := pipelineDB.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(pipelineDB.Config()).To(Equal(updatedConfig))
			Expect(pipelineDB.ConfigVersion()).NotTo(Equal(0))

			found, err = otherPipelineDB.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(otherPipelineDB.Config()).To(Equal(updatedConfig))
			Expect(otherPipelineDB.ConfigVersion()).NotTo(Equal(0))
		})
	})

	Context("Resources", func() {
		It("initially reports zero builds for a job", func() {
			builds, err := pipelineDB.GetAllJobBuilds("some-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(builds).To(BeEmpty())
		})
	})

	Describe("Jobs", func() {
		Describe("GetDashboard", func() {
			It("returns a Dashboard object with a DashboardJob corresponding to each configured job", func() {
				pipelineDB.UpdateFirstLoggedBuildID("some-job", 57)

				job, found, err := pipelineDB.GetJob("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				otherJob, found, err := pipelineDB.GetJob("some-other-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				aJob, found, err := pipelineDB.GetJob("a-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				sharedJob, found, err := pipelineDB.GetJob("shared-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				randomJob, found, err := pipelineDB.GetJob("random-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				otherSerialGroupJob, found, err := pipelineDB.GetJob("other-serial-group-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				differentSerialGroupJob, found, err := pipelineDB.GetJob("different-serial-group-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				By("returning jobs with no builds")
				expectedDashboard := db.Dashboard{
					{
						Job:           job,
						NextBuild:     nil,
						FinishedBuild: nil,
					},
					{
						Job:           otherJob,
						NextBuild:     nil,
						FinishedBuild: nil,
					},
					{
						Job:           aJob,
						NextBuild:     nil,
						FinishedBuild: nil,
					},
					{
						Job:           sharedJob,
						NextBuild:     nil,
						FinishedBuild: nil,
					},
					{
						Job:           randomJob,
						NextBuild:     nil,
						FinishedBuild: nil,
					},
					{
						Job:           otherSerialGroupJob,
						NextBuild:     nil,
						FinishedBuild: nil,
					},
					{
						Job:           differentSerialGroupJob,
						NextBuild:     nil,
						FinishedBuild: nil,
					},
				}

				actualDashboard, groups, err := pipelineDB.GetDashboard()
				Expect(err).NotTo(HaveOccurred())

				Expect(groups).To(Equal(pipelineConfig.Groups))
				Expect(actualDashboard).To(Equal(expectedDashboard))

				By("returning a job's most recent pending build if there are no running builds")
				jobBuildOldDB, err := pipelineDB.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				expectedDashboard[0].NextBuild = jobBuildOldDB

				actualDashboard, _, err = pipelineDB.GetDashboard()
				Expect(err).NotTo(HaveOccurred())

				Expect(actualDashboard).To(Equal(expectedDashboard))

				By("returning a job's most recent started build")
				jobBuildOldDB.Start("engine", "metadata")

				found, err = jobBuildOldDB.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				expectedDashboard[0].NextBuild = jobBuildOldDB

				actualDashboard, _, err = pipelineDB.GetDashboard()
				Expect(err).NotTo(HaveOccurred())

				Expect(actualDashboard).To(Equal(expectedDashboard))

				By("returning a job's most recent started build even if there is a newer pending build")
				jobBuild, err := pipelineDB.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				expectedDashboard[0].NextBuild = jobBuildOldDB

				actualDashboard, _, err = pipelineDB.GetDashboard()
				Expect(err).NotTo(HaveOccurred())

				Expect(actualDashboard).To(Equal(expectedDashboard))

				By("returning a job's most recent finished build")
				err = jobBuild.Finish(db.StatusSucceeded)
				Expect(err).NotTo(HaveOccurred())

				found, err = jobBuild.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				expectedDashboard[0].FinishedBuild = jobBuild
				expectedDashboard[0].NextBuild = jobBuildOldDB

				actualDashboard, _, err = pipelineDB.GetDashboard()
				Expect(err).NotTo(HaveOccurred())

				Expect(actualDashboard).To(Equal(expectedDashboard))

				By("returning a job's most recent finished build even when there is a newer unfinished build")
				jobBuildNewDB, err := pipelineDB.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
				jobBuildNewDB.Start("engine", "metadata")
				found, err = jobBuildNewDB.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				expectedDashboard[0].FinishedBuild = jobBuild
				expectedDashboard[0].NextBuild = jobBuildNewDB

				actualDashboard, _, err = pipelineDB.GetDashboard()
				Expect(err).NotTo(HaveOccurred())

				Expect(actualDashboard).To(Equal(expectedDashboard))
			})
		})

		Describe("pausing and unpausing jobs", func() {
			job := "some-job"

			It("starts out as unpaused", func() {
				job, found, err := pipelineDB.GetJob(job)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(job.Paused).To(BeFalse())
			})

			It("can be paused", func() {
				err := pipelineDB.PauseJob(job)
				Expect(err).NotTo(HaveOccurred())

				err = otherPipelineDB.UnpauseJob(job)
				Expect(err).NotTo(HaveOccurred())

				pausedJob, found, err := pipelineDB.GetJob(job)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(pausedJob.Paused).To(BeTrue())

				otherJob, found, err := otherPipelineDB.GetJob(job)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(otherJob.Paused).To(BeFalse())
			})

			It("can be unpaused", func() {
				err := pipelineDB.PauseJob(job)
				Expect(err).NotTo(HaveOccurred())

				err = pipelineDB.UnpauseJob(job)
				Expect(err).NotTo(HaveOccurred())

				unpausedJob, found, err := pipelineDB.GetJob(job)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(unpausedJob.Paused).To(BeFalse())
			})
		})

		Describe("UpdateFirstLoggedBuildID", func() {
			It("updates FirstLoggedBuildID on a job", func() {
				By("starting out as 0")
				job, found, err := pipelineDB.GetJob("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(job.FirstLoggedBuildID).To(BeZero())

				By("increasing it to 57")

				err = pipelineDB.UpdateFirstLoggedBuildID("some-job", 57)
				Expect(err).NotTo(HaveOccurred())

				updatedJob, found, err := pipelineDB.GetJob("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(updatedJob.FirstLoggedBuildID).To(Equal(57))

				By("not erroring when it's called with the same number")
				err = pipelineDB.UpdateFirstLoggedBuildID("some-job", 57)
				Expect(err).NotTo(HaveOccurred())

				By("erroring when the number decreases")
				err = pipelineDB.UpdateFirstLoggedBuildID("some-job", 56)
				Expect(err).To(Equal(db.FirstLoggedBuildIDDecreasedError{
					Job:   "some-job",
					OldID: 57,
					NewID: 56,
				}))
			})
		})

		Describe("GetJobBuild", func() {
			var firstBuild db.Build
			var job db.SavedJob

			BeforeEach(func() {
				var err error
				var found bool
				job, found, err = pipelineDB.GetJob("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				firstBuild, err = pipelineDB.CreateJobBuild(job.Name)
				Expect(err).NotTo(HaveOccurred())
			})

			It("finds the build", func() {
				build, found, err := pipelineDB.GetJobBuild(job.Name, firstBuild.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.ID()).To(Equal(firstBuild.ID()))
				Expect(build.Status()).To(Equal(firstBuild.Status()))
			})
		})

		Describe("GetNextPendingBuildBySerialGroup", func() {
			var jobOneConfig atc.JobConfig
			var jobOneTwoConfig atc.JobConfig

			BeforeEach(func() {
				jobOneConfig = pipelineConfig.Jobs[0]    //serial-group
				jobOneTwoConfig = pipelineConfig.Jobs[5] //serial-group, really-different-group
			})

			Context("when some jobs have builds with inputs determined as false", func() {
				var actualBuild db.Build

				BeforeEach(func() {
					_, err := pipelineDB.CreateJobBuild(jobOneConfig.Name)
					Expect(err).NotTo(HaveOccurred())

					actualBuild, err = pipelineDB.CreateJobBuild(jobOneTwoConfig.Name)
					Expect(err).NotTo(HaveOccurred())

					err = pipelineDB.SaveNextInputMapping(nil, "other-serial-group-job")
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return the next most pending build in a group of jobs", func() {
					build, found, err := pipelineDB.GetNextPendingBuildBySerialGroup(jobOneConfig.Name, []string{"serial-group"})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(build.ID()).To(Equal(actualBuild.ID()))
				})
			})

			It("should return the next most pending build in a group of jobs", func() {
				buildOne, err := pipelineDB.CreateJobBuild(jobOneConfig.Name)
				Expect(err).NotTo(HaveOccurred())

				buildTwo, err := pipelineDB.CreateJobBuild(jobOneConfig.Name)
				Expect(err).NotTo(HaveOccurred())

				buildThree, err := pipelineDB.CreateJobBuild(jobOneTwoConfig.Name)
				Expect(err).NotTo(HaveOccurred())

				err = pipelineDB.SaveNextInputMapping(nil, "some-job")
				Expect(err).NotTo(HaveOccurred())
				err = pipelineDB.SaveNextInputMapping(nil, "other-serial-group-job")
				Expect(err).NotTo(HaveOccurred())

				build, found, err := pipelineDB.GetNextPendingBuildBySerialGroup(jobOneConfig.Name, []string{"serial-group"})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.ID()).To(Equal(buildOne.ID()))

				build, found, err = pipelineDB.GetNextPendingBuildBySerialGroup(jobOneTwoConfig.Name, []string{"serial-group", "really-different-group"})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.ID()).To(Equal(buildOne.ID()))

				Expect(buildOne.Finish(db.StatusSucceeded)).To(Succeed())

				build, found, err = pipelineDB.GetNextPendingBuildBySerialGroup(jobOneConfig.Name, []string{"serial-group"})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.ID()).To(Equal(buildTwo.ID()))

				build, found, err = pipelineDB.GetNextPendingBuildBySerialGroup(jobOneTwoConfig.Name, []string{"serial-group", "really-different-group"})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.ID()).To(Equal(buildTwo.ID()))

				scheduled, err := pipelineDB.UpdateBuildToScheduled(buildTwo.ID())
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduled).To(BeTrue())
				Expect(buildTwo.Finish(db.StatusSucceeded)).To(Succeed())

				build, found, err = pipelineDB.GetNextPendingBuildBySerialGroup(jobOneConfig.Name, []string{"serial-group"})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.ID()).To(Equal(buildThree.ID()))

				build, found, err = pipelineDB.GetNextPendingBuildBySerialGroup(jobOneTwoConfig.Name, []string{"serial-group", "really-different-group"})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.ID()).To(Equal(buildThree.ID()))
			})
		})

		Describe("GetRunningBuildsBySerialGroup", func() {
			Describe("same job", func() {
				var startedBuild, scheduledBuild db.Build

				BeforeEach(func() {
					var err error
					_, err = pipelineDB.CreateJobBuild("some-job")
					Expect(err).NotTo(HaveOccurred())

					startedBuild, err = pipelineDB.CreateJobBuild("some-job")
					Expect(err).NotTo(HaveOccurred())
					_, err = startedBuild.Start("", "")
					Expect(err).NotTo(HaveOccurred())

					scheduledBuild, err = pipelineDB.CreateJobBuild("some-job")
					Expect(err).NotTo(HaveOccurred())

					scheduled, err := pipelineDB.UpdateBuildToScheduled(scheduledBuild.ID())
					Expect(err).NotTo(HaveOccurred())
					Expect(scheduled).To(BeTrue())

					for _, s := range []db.Status{db.StatusSucceeded, db.StatusFailed, db.StatusErrored, db.StatusAborted} {
						finishedBuild, err := pipelineDB.CreateJobBuild("some-job")
						Expect(err).NotTo(HaveOccurred())

						scheduled, err = pipelineDB.UpdateBuildToScheduled(finishedBuild.ID())
						Expect(err).NotTo(HaveOccurred())
						Expect(scheduled).To(BeTrue())

						err = finishedBuild.Finish(s)
						Expect(err).NotTo(HaveOccurred())
					}

					_, err = pipelineDB.CreateJobBuild("some-other-job")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns a list of running or schedule builds for said job", func() {
					builds, err := pipelineDB.GetRunningBuildsBySerialGroup("some-job", []string{"serial-group"})
					Expect(err).NotTo(HaveOccurred())

					Expect(len(builds)).To(Equal(2))
					ids := []int{}
					for _, build := range builds {
						ids = append(ids, build.ID())
					}
					Expect(ids).To(ConsistOf([]int{startedBuild.ID(), scheduledBuild.ID()}))
				})
			})

			Describe("multiple jobs with same serial group", func() {
				var serialGroupBuild db.Build

				BeforeEach(func() {
					var err error
					_, err = pipelineDB.CreateJobBuild("some-job")
					Expect(err).NotTo(HaveOccurred())

					serialGroupBuild, err = pipelineDB.CreateJobBuild("other-serial-group-job")
					Expect(err).NotTo(HaveOccurred())

					scheduled, err := pipelineDB.UpdateBuildToScheduled(serialGroupBuild.ID())
					Expect(err).NotTo(HaveOccurred())
					Expect(scheduled).To(BeTrue())

					differentSerialGroupBuild, err := pipelineDB.CreateJobBuild("different-serial-group-job")
					Expect(err).NotTo(HaveOccurred())

					scheduled, err = pipelineDB.UpdateBuildToScheduled(differentSerialGroupBuild.ID())
					Expect(err).NotTo(HaveOccurred())
					Expect(scheduled).To(BeTrue())
				})

				It("returns a list of builds in the same serial group", func() {
					builds, err := pipelineDB.GetRunningBuildsBySerialGroup("some-job", []string{"serial-group"})
					Expect(err).NotTo(HaveOccurred())

					Expect(len(builds)).To(Equal(1))
					Expect(builds[0].ID()).To(Equal(serialGroupBuild.ID()))
				})
			})
		})

		It("can report a job's latest running and finished builds", func() {
			finished, next, err := pipelineDB.GetJobFinishedAndNextBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			Expect(next).To(BeNil())
			Expect(finished).To(BeNil())

			finishedBuild, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			err = finishedBuild.Finish(db.StatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			otherFinishedBuild, err := otherPipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			err = otherFinishedBuild.Finish(db.StatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			finished, next, err = pipelineDB.GetJobFinishedAndNextBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			Expect(next).To(BeNil())
			Expect(finished.ID()).To(Equal(finishedBuild.ID()))

			nextBuild, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			started, err := nextBuild.Start("some-engine", "meta")
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			otherNextBuild, err := otherPipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			otherStarted, err := otherNextBuild.Start("some-engine", "meta")
			Expect(err).NotTo(HaveOccurred())
			Expect(otherStarted).To(BeTrue())

			finished, next, err = pipelineDB.GetJobFinishedAndNextBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			Expect(next.ID()).To(Equal(nextBuild.ID()))
			Expect(finished.ID()).To(Equal(finishedBuild.ID()))

			anotherRunningBuild, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			finished, next, err = pipelineDB.GetJobFinishedAndNextBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			Expect(next.ID()).To(Equal(nextBuild.ID())) // not anotherRunningBuild
			Expect(finished.ID()).To(Equal(finishedBuild.ID()))

			started, err = anotherRunningBuild.Start("some-engine", "meta")
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			finished, next, err = pipelineDB.GetJobFinishedAndNextBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			Expect(next.ID()).To(Equal(nextBuild.ID())) // not anotherRunningBuild
			Expect(finished.ID()).To(Equal(finishedBuild.ID()))

			err = nextBuild.Finish(db.StatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			finished, next, err = pipelineDB.GetJobFinishedAndNextBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			Expect(next.ID()).To(Equal(anotherRunningBuild.ID()))
			Expect(finished.ID()).To(Equal(nextBuild.ID()))
		})
	})
})
