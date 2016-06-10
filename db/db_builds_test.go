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

		buildDBFactory = db.NewBuildDBFactory(dbConn, bus)
		teamDBFactory = db.NewTeamDBFactory(dbConn, buildDBFactory)
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
			oneOffBuildDB db.BuildDB
			err           error
		)

		BeforeEach(func() {
			oneOffBuildDB, err = teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		It("can update a builds build preparation", func() {
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
		var buildDB db.BuildDB
		var originalBuildPrep db.BuildPreparation

		BeforeEach(func() {
			var err error
			buildDB, err = pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			originalBuildPrep = db.BuildPreparation{
				BuildID:          buildDB.GetID(),
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
			buildPrep, found, err := buildDB.GetPreparation()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			expectedBuildPrep := db.NewBuildPreparation(buildDB.GetID())
			expectedBuildPrep.PausedPipeline = db.BuildPreparationStatusBlocking
			Expect(buildPrep).To(Equal(expectedBuildPrep))
		})

		Context("where the build is scheduled", func() {
			BeforeEach(func() {
				scheduled, err := pipelineDB.UpdateBuildToScheduled(buildDB.GetID())
				Expect(err).NotTo(HaveOccurred())
				Expect(scheduled).To(BeTrue())
			})

			It("does not update scheduled build's build prep", func() {
				buildPrep, found, err := buildDB.GetPreparation()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(buildPrep).To(Equal(originalBuildPrep))
			})
		})
	})

	Describe("GetAllStartedBuilds", func() {
		var build1DB db.BuildDB
		var build2DB db.BuildDB

		BeforeEach(func() {
			var err error
			build1DB, err = teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			build2DB, err = pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			_, err = teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			started, err := build1DB.Start("some-engine", "so-meta")
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			started, err = build2DB.Start("some-engine", "so-meta")
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())
		})

		It("returns all builds that have been started, regardless of pipeline", func() {
			buildDBs, err := database.GetAllStartedBuilds()
			Expect(err).NotTo(HaveOccurred())

			build1DB.Reload()
			build2DB.Reload()
			Expect(buildDBs).To(ConsistOf(build1DB, build2DB))
		})
	})

	Describe("DeleteBuildEventsByBuildIDs", func() {
		It("deletes all build logs corresponding to the given build ids", func() {
			build1DB, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			err = build1DB.SaveEvent(event.Log{
				Payload: "log 1",
			})
			Expect(err).NotTo(HaveOccurred())

			build2DB, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			err = build2DB.SaveEvent(event.Log{
				Payload: "log 2",
			})
			Expect(err).NotTo(HaveOccurred())

			build3DB, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			err = build3DB.Finish(db.StatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			err = build1DB.Finish(db.StatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			err = build2DB.Finish(db.StatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			build4DB, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			By("doing nothing if the list is empty")
			err = database.DeleteBuildEventsByBuildIDs([]int{})
			Expect(err).NotTo(HaveOccurred())

			By("not returning an error")
			database.DeleteBuildEventsByBuildIDs([]int{build3DB.GetID(), build4DB.GetID(), build1DB.GetID()})
			Expect(err).NotTo(HaveOccurred())

			err = build4DB.Finish(db.StatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			By("deleting events for build 1")
			events1, err := build1DB.Events(0)
			Expect(err).NotTo(HaveOccurred())
			defer events1.Close()

			_, err = events1.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("preserving events for build 2")
			events2, err := build2DB.Events(0)
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
			events3, err := build3DB.Events(0)
			Expect(err).NotTo(HaveOccurred())
			defer events3.Close()

			_, err = events3.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("being unflapped by build 4, which had no events at the time")
			events4, err := build4DB.Events(0)
			Expect(err).NotTo(HaveOccurred())
			defer events4.Close()

			_, err = events4.Next() // finish event
			Expect(err).NotTo(HaveOccurred())

			_, err = events4.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("updating ReapTime for the affected builds")
			found, err := build1DB.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(build1DB.GetReapTime()).To(BeTemporally(">", build1DB.GetEndTime()))

			found, err = build2DB.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(build2DB.GetReapTime()).To(BeZero())

			found, err = build3DB.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(build3DB.GetReapTime()).To(Equal(build1DB.GetReapTime()))

			found, err = build4DB.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			// Not required behavior, just a sanity check for what I think will happen
			Expect(build4DB.GetReapTime()).To(Equal(build1DB.GetReapTime()))
		})
	})
})
