package db_test

import (
	"time"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
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

	Describe("UpdateBuildPreparation", func() {
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

	Describe("DeleteBuildEventsByBuildIDs", func() {
		It("deletes all build logs corresponding to the given build ids", func() {
			build1, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			build1DB := buildDBFactory.GetBuildDB(build1)
			err = build1DB.SaveEvent(event.Log{
				Payload: "log 1",
			})
			Expect(err).NotTo(HaveOccurred())

			build2, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			build2DB := buildDBFactory.GetBuildDB(build2)
			err = build2DB.SaveEvent(event.Log{
				Payload: "log 2",
			})
			Expect(err).NotTo(HaveOccurred())

			build3, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			build3DB := buildDBFactory.GetBuildDB(build3)
			err = build3DB.Finish(db.StatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			err = build1DB.Finish(db.StatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			err = build2DB.Finish(db.StatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			build4, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			By("doing nothing if the list is empty")
			err = database.DeleteBuildEventsByBuildIDs([]int{})
			Expect(err).NotTo(HaveOccurred())

			By("not returning an error")
			database.DeleteBuildEventsByBuildIDs([]int{build3.ID, build4.ID, build1.ID})
			Expect(err).NotTo(HaveOccurred())

			build4DB := buildDBFactory.GetBuildDB(build4)
			err = build4DB.Finish(db.StatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			By("deleting events for build 1")
			events1, err := buildDBFactory.GetBuildDB(build1).Events(0)
			Expect(err).NotTo(HaveOccurred())
			defer events1.Close()

			_, err = events1.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("preserving events for build 2")
			events2, err := buildDBFactory.GetBuildDB(build2).Events(0)
			Expect(err).NotTo(HaveOccurred())
			defer events2.Close()

			build2Event1, err := events2.Next()
			Expect(err).NotTo(HaveOccurred())
			Expect(build2Event1).To(Equal(event.Log{
				Payload: "log 2",
			}))

			_, err = events2.Next() // finish event
			Expect(err).NotTo(HaveOccurred())

			_, err = events2.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("deleting events for build 3")
			events3, err := buildDBFactory.GetBuildDB(build3).Events(0)
			Expect(err).NotTo(HaveOccurred())
			defer events3.Close()

			_, err = events3.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("being unflapped by build 4, which had no events at the time")
			events4, err := buildDBFactory.GetBuildDB(build4).Events(0)
			Expect(err).NotTo(HaveOccurred())
			defer events4.Close()

			_, err = events4.Next() // finish event
			Expect(err).NotTo(HaveOccurred())

			_, err = events4.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("updating ReapTime for the affected builds")

			reapedBuild1, found, err := teamDB.GetBuild(build1.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(reapedBuild1.ReapTime).To(BeTemporally(">", reapedBuild1.EndTime))

			reapedBuild2, found, err := teamDB.GetBuild(build2.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(reapedBuild2.ReapTime).To(BeZero())

			reapedBuild3, found, err := teamDB.GetBuild(build3.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(reapedBuild3.ReapTime).To(Equal(reapedBuild1.ReapTime))

			reapedBuild4, found, err := teamDB.GetBuild(build4.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			// Not required behavior, just a sanity check for what I think will happen
			Expect(reapedBuild4.ReapTime).To(Equal(reapedBuild1.ReapTime))
		})
	})
})
