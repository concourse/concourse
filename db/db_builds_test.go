package db_test

import (
	"time"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("Keeping track of builds", func() {
	var dbConn db.Conn
	var listener *pq.Listener
	var sqlDB db.DB

	var database db.DB
	var pipelineDB db.PipelineDB
	var pipeline db.SavedPipeline
	var teamDBFactory db.TeamDBFactory
	var team db.SavedTeam
	var teamDB db.TeamDB
	var buildDBFactory db.BuildDBFactory

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		sqlDB = db.NewSQL(dbConn, bus)

		var err error
		team, err = sqlDB.CreateTeam(db.Team{Name: "some-team"})
		Expect(err).NotTo(HaveOccurred())
		teamDBFactory = db.NewTeamDBFactory(dbConn)
		teamDB = teamDBFactory.GetTeamDB("some-team")

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

		pipeline, _, err = teamDB.SaveConfig("some-pipeline", config, db.ConfigVersion(1), db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDBFactory := db.NewPipelineDBFactory(dbConn, bus)
		pipelineDB = pipelineDBFactory.Build(pipeline)

		buildDBFactory = db.NewBuildDBFactory(dbConn, bus)

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

		expectedBuildInput, err := pipelineDB.SaveInput(build.ID, db.BuildInput{
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

		actualBuildInput, err := buildDBFactory.GetBuildDB(build).GetVersionedResources()
		expectedBuildInput.CheckOrder = 0
		Expect(err).ToNot(HaveOccurred())
		Expect(len(actualBuildInput)).To(Equal(1))
		Expect(actualBuildInput[0]).To(Equal(expectedBuildInput))
	})

	It("can get a build's output", func() {
		build, err := pipelineDB.CreateJobBuild("some-job")
		Expect(err).ToNot(HaveOccurred())

		expectedBuildOutput, err := pipelineDB.SaveOutput(build.ID, db.VersionedResource{
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

		_, err = pipelineDB.SaveOutput(build.ID, db.VersionedResource{
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

		actualBuildOutput, err := buildDBFactory.GetBuildDB(build).GetVersionedResources()
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
			oneOff, err = teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		It("can get (no) resources from a one-off build", func() {
			inputs, outputs, err := buildDBFactory.GetBuildDB(oneOff).GetResources()
			Expect(err).NotTo(HaveOccurred())

			Expect(inputs).To(BeEmpty())
			Expect(outputs).To(BeEmpty())
		})

		It("can create one-off builds with increasing names", func() {
			Expect(oneOff.ID).NotTo(BeZero())
			Expect(oneOff.JobName).To(BeZero())
			Expect(oneOff.Name).To(Equal("1"))
			Expect(oneOff.TeamID).To(Equal(team.ID))
			Expect(oneOff.TeamName).To(Equal(team.Name))
			Expect(oneOff.Status).To(Equal(db.StatusPending))

			oneOffGot, found, err := teamDB.GetBuild(oneOff.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(oneOffGot).To(Equal(oneOff))

			jobBuild, err := pipelineDB.CreateJobBuild("some-other-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(jobBuild.Name).To(Equal("1"))

			nextOneOff, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			Expect(nextOneOff.Name).To(Equal("2"))

			allBuilds, _, err := teamDB.GetBuilds(db.Page{Limit: 100})
			Expect(err).NotTo(HaveOccurred())
			Expect(allBuilds).To(Equal([]db.Build{nextOneOff, jobBuild, oneOff}))
		})

		It("also creates buildpreparation", func() {
			buildPrep, found, err := buildDBFactory.GetBuildDB(oneOff).GetPreparation()
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
			oneOff, err = teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		It("can update a builds build preparation", func() {
			oneOffBuildDB := buildDBFactory.GetBuildDB(oneOff)
			buildPrep, found, err := oneOffBuildDB.GetPreparation()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			buildPrep.PausedPipeline = db.BuildPreparationStatusBlocking
			buildPrep.Inputs["banana"] = "doesnt matter"
			buildPrep.InputsSatisfied = db.BuildPreparationStatusNotBlocking
			buildPrep.MissingInputReasons = map[string]string{"some-input": "some missing reason"}

			err = database.UpdateBuildPreparation(buildPrep)
			Expect(err).NotTo(HaveOccurred())

			newBuildPrep, found, err := oneOffBuildDB.GetPreparation()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(newBuildPrep).To(Equal(buildPrep))
		})
	})

	Describe("ResetBuildPreparationWithPipelinePaused", func() {
		var buildID int
		var build db.Build
		var originalBuildPrep db.BuildPreparation

		BeforeEach(func() {
			var err error
			build, err = pipelineDB.CreateJobBuild("some-job")
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
			buildPrep, found, err := buildDBFactory.GetBuildDB(build).GetPreparation()
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
				buildPrep, found, err := buildDBFactory.GetBuildDB(build).GetPreparation()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(buildPrep).To(Equal(originalBuildPrep))
			})
		})
	})

	Describe("GetAllStartedBuilds", func() {
		var build1 db.Build
		var build2 db.Build

		BeforeEach(func() {
			var err error

			build1, err = teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			build2, err = pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			_, err = teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			started, err := buildDBFactory.GetBuildDB(build1).Start("some-engine", "so-meta")
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			build2DB := buildDBFactory.GetBuildDB(build2)
			started, err = build2DB.Start("some-engine", "so-meta")
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())
		})

		It("returns all builds that have been started, regardless of pipeline", func() {
			builds, err := database.GetAllStartedBuilds()
			Expect(err).NotTo(HaveOccurred())

			Expect(len(builds)).To(Equal(2))

			build1, found, err := teamDB.GetBuild(build1.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			build2, found, err := teamDB.GetBuild(build2.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(builds).To(ConsistOf(build1, build2))
		})
	})
})
