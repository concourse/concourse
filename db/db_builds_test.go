package db_test

import (
	"time"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func createAndFinishBuild(database db.DB, pipelineDB db.PipelineDB, jobName string, status db.Status) db.Build {
	build, err := pipelineDB.CreateJobBuild(jobName)
	Expect(err).NotTo(HaveOccurred())

	err = database.FinishBuild(build.ID, build.PipelineID, status)
	Expect(err).NotTo(HaveOccurred())

	return build
}

func createAndStartBuild(database db.DB, pipelineDB db.PipelineDB, jobName string, engineName string) db.Build {
	build, err := pipelineDB.CreateJobBuild(jobName)
	Expect(err).NotTo(HaveOccurred())

	started, err := database.StartBuild(build.ID, build.PipelineID, engineName, "so-meta")
	Expect(started).To(BeTrue())
	Expect(err).NotTo(HaveOccurred())

	return build
}

var _ = Describe("Keeping track of builds", func() {
	var (
		err               error
		dbConn            db.Conn
		listener          *pq.Listener
		database          db.DB
		sqlDB             *db.SQLDB
		pipelineDBFactory db.PipelineDBFactory
		pipelineDB        db.PipelineDB
		pipeline          db.SavedPipeline
		team              db.SavedTeam
		config            atc.Config
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		sqlDB = db.NewSQL(dbConn, bus)

		pipelineDBFactory = db.NewPipelineDBFactory(dbConn, bus, sqlDB)

		team, err = sqlDB.SaveTeam(db.Team{Name: "some-team"})
		Expect(err).NotTo(HaveOccurred())

		config = atc.Config{
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
				{
					Name: "some-implicit-resource",
					Type: "some-type",
				},
				{
					Name: "some-explicit-resource",
					Type: "some-type",
				},
			},
		}

		pipeline, _, err = sqlDB.SaveConfig(team.Name, "some-pipeline", config, db.ConfigVersion(1), db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())
		pipelineDB, err = pipelineDBFactory.BuildWithTeamNameAndName(team.Name, "some-pipeline")
		Expect(err).NotTo(HaveOccurred())

		database = sqlDB
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	It("can get a build's inputs", func() {
		build, err := pipelineDB.CreateJobBuild("some-job")
		Expect(err).ToNot(HaveOccurred())

		expectedBuildInput, err := pipelineDB.SaveBuildInput(build.ID, db.BuildInput{
			Name: "some-input",
			VersionedResource: db.VersionedResource{
				Resource: "some-resource",
				Type:     "some-type",
				Version: db.Version{
					"some": "version",
				},
				Metadata: []db.MetadataField{
					{
						Name:  "meta1",
						Value: "data1",
					},
					{
						Name:  "meta2",
						Value: "data2",
					},
				},
				PipelineID: pipeline.ID,
			},
		})
		Expect(err).ToNot(HaveOccurred())

		actualBuildInput, err := database.GetBuildVersionedResources(build.ID)
		expectedBuildInput.CheckOrder = 0
		Expect(err).ToNot(HaveOccurred())
		Expect(len(actualBuildInput)).To(Equal(1))
		Expect(actualBuildInput[0]).To(Equal(expectedBuildInput))
	})

	It("can get a build's output", func() {
		build, err := pipelineDB.CreateJobBuild("some-job")
		Expect(err).ToNot(HaveOccurred())

		expectedBuildOutput, err := pipelineDB.SaveBuildOutput(build.ID, db.VersionedResource{
			Resource: "some-explicit-resource",
			Type:     "some-type",
			Version: db.Version{
				"some": "version",
			},
			Metadata: []db.MetadataField{
				{
					Name:  "meta1",
					Value: "data1",
				},
				{
					Name:  "meta2",
					Value: "data2",
				},
			},
			PipelineID: pipeline.ID,
		}, true)
		Expect(err).ToNot(HaveOccurred())

		_, err = pipelineDB.SaveBuildOutput(build.ID, db.VersionedResource{
			Resource: "some-implicit-resource",
			Type:     "some-type",
			Version: db.Version{
				"some": "version",
			},
			Metadata: []db.MetadataField{
				{
					Name:  "meta1",
					Value: "data1",
				},
				{
					Name:  "meta2",
					Value: "data2",
				},
			},
			PipelineID: pipeline.ID,
		}, false)
		Expect(err).ToNot(HaveOccurred())

		actualBuildOutput, err := database.GetBuildVersionedResources(build.ID)
		expectedBuildOutput.CheckOrder = 0
		Expect(err).ToNot(HaveOccurred())
		Expect(len(actualBuildOutput)).To(Equal(1))
		Expect(actualBuildOutput[0]).To(Equal(expectedBuildOutput))
	})

	Context("build creation", func() {
		var (
			oneOff db.Build
			err    error
		)

		BeforeEach(func() {
			oneOff, err = database.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		It("can get (no) resources from a one-off build", func() {
			inputs, outputs, err := database.GetBuildResources(oneOff.ID)
			Expect(err).NotTo(HaveOccurred())

			Expect(inputs).To(BeEmpty())
			Expect(outputs).To(BeEmpty())
		})

		It("can create one-off builds with increasing names", func() {
			Expect(oneOff.ID).NotTo(BeZero())
			Expect(oneOff.JobName).To(BeZero())
			Expect(oneOff.Name).To(Equal("1"))
			Expect(oneOff.Status).To(Equal(db.StatusPending))

			oneOffGot, found, err := database.GetBuild(oneOff.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(oneOffGot).To(Equal(oneOff))

			jobBuild, err := pipelineDB.CreateJobBuild("some-other-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(jobBuild.Name).To(Equal("1"))

			nextOneOff, err := database.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			Expect(nextOneOff.ID).NotTo(BeZero())
			Expect(nextOneOff.ID).NotTo(Equal(oneOff.ID))
			Expect(nextOneOff.JobName).To(BeZero())
			Expect(nextOneOff.Name).To(Equal("2"))
			Expect(nextOneOff.Status).To(Equal(db.StatusPending))

			allBuilds, _, err := database.GetBuilds(db.Page{Limit: 100})
			Expect(err).NotTo(HaveOccurred())
			Expect(allBuilds).To(Equal([]db.Build{nextOneOff, jobBuild, oneOff}))
		})

		It("also creates buildpreparation", func() {
			buildPrep, found, err := database.GetBuildPreparation(oneOff.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(buildPrep.BuildID).To(Equal(oneOff.ID))
		})
	})

	Describe("build preparation update", func() {
		var (
			oneOff db.Build
			err    error
		)
		BeforeEach(func() {
			oneOff, err = database.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		It("can update a builds build preparation", func() {
			buildPrep, found, err := database.GetBuildPreparation(oneOff.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			buildPrep.PausedPipeline = db.BuildPreparationStatusBlocking
			buildPrep.Inputs["banana"] = "doesnt matter"
			buildPrep.InputsSatisfied = db.BuildPreparationStatusNotBlocking
			buildPrep.MissingInputReasons = map[string]string{"some-input": "some missing reason"}

			err = database.UpdateBuildPreparation(buildPrep)
			Expect(err).NotTo(HaveOccurred())

			newBuildPrep, found, err := database.GetBuildPreparation(oneOff.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(newBuildPrep).To(Equal(buildPrep))
		})
	})

	Describe("ResetBuildPreparationWithPipelinePaused", func() {
		var buildID int
		var originalBuildPrep db.BuildPreparation

		BeforeEach(func() {
			build, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())
			buildID = build.ID

			originalBuildPrep = db.BuildPreparation{
				BuildID:          buildID,
				PausedPipeline:   db.BuildPreparationStatusNotBlocking,
				PausedJob:        db.BuildPreparationStatusNotBlocking,
				MaxRunningBuilds: db.BuildPreparationStatusNotBlocking,
				Inputs: map[string]db.BuildPreparationStatus{
					"banana": db.BuildPreparationStatusNotBlocking,
					"potato": db.BuildPreparationStatusNotBlocking,
				},
				InputsSatisfied:     db.BuildPreparationStatusBlocking,
				MissingInputReasons: map[string]string{},
			}

			err = pipelineDB.UpdateBuildPreparation(originalBuildPrep)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			err := database.ResetBuildPreparationsWithPipelinePaused(pipeline.ID)
			Expect(err).NotTo(HaveOccurred())
		})

		It("resets the build prep and marks the pipeline as blocking", func() {
			buildPrep, found, err := database.GetBuildPreparation(buildID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			expectedBuildPrep := db.NewBuildPreparation(buildID)
			expectedBuildPrep.PausedPipeline = db.BuildPreparationStatusBlocking
			Expect(buildPrep).To(Equal(expectedBuildPrep))
		})

		Context("where the build is scheduled", func() {
			BeforeEach(func() {
				scheduled, err := pipelineDB.UpdateBuildToScheduled(buildID)
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduled).To(BeTrue())
			})

			It("does not update scheduled build's build prep", func() {
				buildPrep, found, err := database.GetBuildPreparation(buildID)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(buildPrep).To(Equal(originalBuildPrep))
			})
		})
	})

	Describe("FindJobIDForBuild", func() {
		var build db.Build
		BeforeEach(func() {
			build = createAndFinishBuild(database, pipelineDB, "some-job", db.StatusSucceeded)
			createAndFinishBuild(database, pipelineDB, "some-job", db.StatusSucceeded)
		})

		It("finds the job id for the given build", func() {
			jobID, found, err := database.FindJobIDForBuild(build.ID)
			Expect(found).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			job, err := pipelineDB.GetJob("some-job")
			Expect(err).NotTo(HaveOccurred())

			Expect(jobID).To(Equal(job.ID))
		})
	})

	Describe("GetLatestFinishedBuildForJob", func() {
		var (
			finishedBuild2  db.Build
			otherPipeline   db.SavedPipeline
			otherPipelineDB db.PipelineDB
			err             error
		)

		BeforeEach(func() {
			otherPipeline, _, err = sqlDB.SaveConfig(team.Name, "some-other-pipeline", config, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			otherPipelineDB, err = pipelineDBFactory.BuildWithTeamNameAndName(team.Name, "some-other-pipeline")
			Expect(err).NotTo(HaveOccurred())

			createAndFinishBuild(database, pipelineDB, "some-job", db.StatusSucceeded)
			createAndStartBuild(database, pipelineDB, "some-job", "some-engine")
			finishedBuild2 = createAndFinishBuild(database, pipelineDB, "some-job", db.StatusSucceeded)
			createAndFinishBuild(database, pipelineDB, "some-other-job", db.StatusSucceeded)
			createAndFinishBuild(database, otherPipelineDB, "some-job", db.StatusSucceeded)
		})

		It("returns the latest finished build of the job", func() {
			job, err := pipelineDB.GetJob("some-job")
			Expect(err).NotTo(HaveOccurred())

			latestFinishedBuild, found, err := database.GetLatestFinishedBuildForJob(job.Name, pipelineDB.GetPipelineID())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			expectedBuild, found, err := database.GetBuild(finishedBuild2.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(latestFinishedBuild).To(Equal(expectedBuild))
		})
	})

	Describe("GetAllStartedBuilds", func() {
		var build1 db.Build
		var build2 db.Build

		BeforeEach(func() {
			var err error

			build1, err = database.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			build2, err = pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			_, err = database.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			started, err := database.StartBuild(build1.ID, build1.PipelineID, "some-engine", "so-meta")
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			started, err = database.StartBuild(build2.ID, build2.PipelineID, "some-engine", "so-meta")
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())
		})

		It("returns all builds that have been started, regardless of pipeline", func() {
			builds, err := database.GetAllStartedBuilds()
			Expect(err).NotTo(HaveOccurred())

			Expect(len(builds)).To(Equal(2))

			build1, found, err := database.GetBuild(build1.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			build2, found, err := database.GetBuild(build2.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(builds).To(ConsistOf(build1, build2))
		})
	})

	Describe("GetBuilds", func() {
		Context("when there are no builds", func() {
			It("returns an empty list of builds", func() {
				builds, pagination, err := database.GetBuilds(db.Page{Limit: 2})
				Expect(err).NotTo(HaveOccurred())

				Expect(pagination.Next).To(BeNil())
				Expect(pagination.Previous).To(BeNil())
				Expect(builds).To(BeEmpty())
			})
		})

		Context("when there are builds", func() {
			var allBuilds [5]db.Build

			BeforeEach(func() {
				for i := 0; i < 3; i++ {
					var err error
					allBuilds[i], err = database.CreateOneOffBuild()
					Expect(err).NotTo(HaveOccurred())
				}

				for i := 3; i < 5; i++ {
					var err error
					allBuilds[i], err = pipelineDB.CreateJobBuild("some-job")
					Expect(err).NotTo(HaveOccurred())
				}
			})

			It("returns all builds that have been started, regardless of pipeline", func() {
				builds, pagination, err := database.GetBuilds(db.Page{Limit: 2})
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(2))
				Expect(builds[0]).To(Equal(allBuilds[4]))
				Expect(builds[1]).To(Equal(allBuilds[3]))

				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(Equal(&db.Page{Since: allBuilds[3].ID, Limit: 2}))

				builds, pagination, err = database.GetBuilds(*pagination.Next)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(2))
				Expect(builds[0]).To(Equal(allBuilds[2]))
				Expect(builds[1]).To(Equal(allBuilds[1]))

				Expect(pagination.Previous).To(Equal(&db.Page{Until: allBuilds[2].ID, Limit: 2}))
				Expect(pagination.Next).To(Equal(&db.Page{Since: allBuilds[1].ID, Limit: 2}))

				builds, pagination, err = database.GetBuilds(*pagination.Next)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(1))
				Expect(builds[0]).To(Equal(allBuilds[0]))

				Expect(pagination.Previous).To(Equal(&db.Page{Until: allBuilds[0].ID, Limit: 2}))
				Expect(pagination.Next).To(BeNil())

				builds, pagination, err = database.GetBuilds(*pagination.Previous)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(builds)).To(Equal(2))
				Expect(builds[0]).To(Equal(allBuilds[2]))
				Expect(builds[1]).To(Equal(allBuilds[1]))

				Expect(pagination.Previous).To(Equal(&db.Page{Until: allBuilds[2].ID, Limit: 2}))
				Expect(pagination.Next).To(Equal(&db.Page{Since: allBuilds[1].ID, Limit: 2}))
			})
		})
	})
})
