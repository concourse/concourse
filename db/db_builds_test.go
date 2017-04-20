package db_test

import (
	"time"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
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

		dbConn = db.Wrap(postgresRunner.OpenDB())
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		lockFactory := lock.NewLockFactory(postgresRunner.OpenSingleton())
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
