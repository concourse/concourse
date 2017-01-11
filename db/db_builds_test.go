package db_test

import (
	"time"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/atc/event"
)

func createAndFinishBuild(database db.DB, pipelineDB db.PipelineDB, jobName string, status db.Status) db.Build {
	build, err := pipelineDB.CreateJobBuild(jobName)
	Expect(err).NotTo(HaveOccurred())

	err = build.Finish(status)
	Expect(err).NotTo(HaveOccurred())

	return build
}

var _ = Describe("Builds", func() {
	var (
		dbConn            db.Conn
		listener          *pq.Listener
		database          db.DB
		pipelineDB        db.PipelineDB
		pipelineDBFactory db.PipelineDBFactory
		pipeline          db.SavedPipeline
		teamDB            db.TeamDB
		config            atc.Config
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		pgxConn := postgresRunner.OpenPgx()
		fakeConnector := new(lockfakes.FakeConnector)
		retryableConn := &lock.RetryableConn{Connector: fakeConnector, Conn: pgxConn}

		lockFactory := lock.NewLockFactory(retryableConn)
		database = db.NewSQL(dbConn, bus, lockFactory)
		_, err := database.CreateTeam(db.Team{Name: "some-team"})
		Expect(err).NotTo(HaveOccurred())

		teamDBFactory := db.NewTeamDBFactory(dbConn, bus, lockFactory)
		teamDB = teamDBFactory.GetTeamDB("some-team")

		config = atc.Config{
			Jobs: atc.JobConfigs{
				{
					Name: "some-job",
				},
				{
					Name: "some-other-job",
				},
				{
					Name: "some-random-job",
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

		pipeline, _, err = teamDB.SaveConfigToBeDeprecated("some-pipeline", config, db.ConfigVersion(1), db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDBFactory = db.NewPipelineDBFactory(dbConn, bus, lockFactory)
		pipelineDB = pipelineDBFactory.Build(pipeline)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	It("can find latest successful builds per job", func() {
		savedBuild0, err := pipelineDB.CreateJobBuild("some-job")
		Expect(err).NotTo(HaveOccurred())

		savedBuild1, err := pipelineDB.CreateJobBuild("some-job")
		Expect(err).NotTo(HaveOccurred())

		savedBuild2, err := pipelineDB.CreateJobBuild("some-other-job")
		Expect(err).NotTo(HaveOccurred())

		savedBuild3, err := pipelineDB.CreateJobBuild("some-random-job")
		Expect(err).NotTo(HaveOccurred())

		err = savedBuild0.Finish(db.StatusSucceeded)
		Expect(err).NotTo(HaveOccurred())

		err = savedBuild1.Finish(db.StatusSucceeded)
		Expect(err).NotTo(HaveOccurred())

		err = savedBuild2.Finish(db.StatusFailed)
		Expect(err).NotTo(HaveOccurred())

		err = savedBuild3.Finish(db.StatusSucceeded)
		Expect(err).NotTo(HaveOccurred())

		someJob, found, err := pipelineDB.GetJob("some-job")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		someRandomJob, found, err := pipelineDB.GetJob("some-random-job")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		jobBuildMap, err := database.FindLatestSuccessfulBuildsPerJob()
		Expect(err).NotTo(HaveOccurred())
		Expect(jobBuildMap).To(Equal(map[int]int{
			someJob.ID:       savedBuild1.ID(),
			someRandomJob.ID: savedBuild3.ID(),
		}))
	})

	Describe("FindJobIDForBuild", func() {
		var build db.Build
		BeforeEach(func() {
			build = createAndFinishBuild(database, pipelineDB, "some-job", db.StatusSucceeded)
			createAndFinishBuild(database, pipelineDB, "some-job", db.StatusSucceeded)
		})

		It("finds the job id for the given build", func() {
			jobID, found, err := database.FindJobIDForBuild(build.ID())
			Expect(found).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			job, found, err := pipelineDB.GetJob("some-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(jobID).To(Equal(job.ID))
		})
	})

	Describe("GetPublicBuilds", func() {
		var publicBuild db.Build

		BeforeEach(func() {
			_, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			config := atc.Config{Jobs: atc.JobConfigs{{Name: "some-job"}}}
			privatePipeline, _, err := teamDB.SaveConfigToBeDeprecated("private-pipeline", config, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())
			privatePipelineDB := pipelineDBFactory.Build(privatePipeline)

			_, err = privatePipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			publicPipeline, _, err := teamDB.SaveConfigToBeDeprecated("public-pipeline", config, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())
			publicPipelineDB := pipelineDBFactory.Build(publicPipeline)
			publicPipelineDB.Expose()

			publicBuild, err = publicPipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns public builds", func() {
			builds, _, err := database.GetPublicBuilds(db.Page{Limit: 10})
			Expect(err).NotTo(HaveOccurred())

			Expect(builds).To(HaveLen(1))
			Expect(builds).To(ConsistOf(publicBuild))
		})
	})

	Describe("GetAllStartedBuilds", func() {
		var build1DB db.Build
		var build2DB db.Build

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
			builds, err := database.GetAllStartedBuilds()
			Expect(err).NotTo(HaveOccurred())

			build1DB.Reload()
			build2DB.Reload()
			Expect(builds).To(ConsistOf(build1DB, build2DB))
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
			database.DeleteBuildEventsByBuildIDs([]int{build3DB.ID(), build4DB.ID(), build1DB.ID()})
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
			Expect(build2Event1).To(Equal(envelope(event.Log{
				Payload: "log 2",
			})))

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

			Expect(build1DB.ReapTime()).To(BeTemporally(">", build1DB.EndTime()))

			found, err = build2DB.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(build2DB.ReapTime()).To(BeZero())

			found, err = build3DB.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(build3DB.ReapTime()).To(Equal(build1DB.ReapTime()))

			found, err = build4DB.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			// Not required behavior, just a sanity check for what I think will happen
			Expect(build4DB.ReapTime()).To(Equal(build1DB.ReapTime()))
		})
	})
})
