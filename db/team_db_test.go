package db_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TeamDB", func() {
	var (
		dbConn   db.Conn
		listener *pq.Listener

		database      db.DB
		teamDBFactory db.TeamDBFactory

		teamDB            db.TeamDB
		otherTeamDB       db.TeamDB
		nonExistentTeamDB db.TeamDB
		savedTeam         db.SavedTeam
		otherSavedTeam    db.SavedTeam
		pipelineDBFactory db.PipelineDBFactory
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.OpenDB())
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		lockFactory := lock.NewLockFactory(postgresRunner.OpenSingleton())
		teamDBFactory = db.NewTeamDBFactory(dbConn, bus, lockFactory)
		database = db.NewSQL(dbConn, bus, lockFactory)

		team := db.Team{Name: "TEAM-name"}
		var err error
		savedTeam, err = database.CreateTeam(team)
		Expect(err).NotTo(HaveOccurred())

		teamDB = teamDBFactory.GetTeamDB("team-NAME")
		nonExistentTeamDB = teamDBFactory.GetTeamDB("non-existent-name")

		pipelineDBFactory = db.NewPipelineDBFactory(dbConn, bus, lockFactory)

		team = db.Team{Name: "other-team-name"}
		otherSavedTeam, err = database.CreateTeam(team)
		Expect(err).NotTo(HaveOccurred())
		otherTeamDB = teamDBFactory.GetTeamDB("other-team-name")
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("GetPipelineByName", func() {
		var savedPipeline db.SavedPipeline
		BeforeEach(func() {
			var err error
			savedPipeline, _, err = teamDB.SaveConfigToBeDeprecated("pipeline-name", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			_, _, err = otherTeamDB.SaveConfigToBeDeprecated("pipeline-name", atc.Config{}, 0, db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the pipeline with the case insensitive name that belongs to the team", func() {
			actualPipeline, found, err := teamDB.GetPipelineByName("pipeline-name")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(actualPipeline).To(Equal(savedPipeline))
		})
	})

	Describe("GetTeam", func() {
		It("returns the saved team", func() {
			actualTeam, found, err := teamDB.GetTeam()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(actualTeam.Name).To(Equal("TEAM-name"))
		})

		It("returns false with no error when the team does not exist", func() {
			_, found, err := nonExistentTeamDB.GetTeam()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Describe("CreateOneOffBuild", func() {
		var (
			oneOffBuild db.Build
			err         error
		)

		BeforeEach(func() {
			oneOffBuild, err = teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		It("can create one-off builds with increasing names", func() {
			nextOneOffBuild, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			Expect(nextOneOffBuild.ID()).NotTo(BeZero())
			Expect(nextOneOffBuild.ID()).NotTo(Equal(oneOffBuild.ID()))
			Expect(nextOneOffBuild.JobName()).To(BeZero())
			Expect(nextOneOffBuild.Name()).To(Equal("2"))
			Expect(nextOneOffBuild.TeamName()).To(Equal(savedTeam.Name))
			Expect(nextOneOffBuild.Status()).To(Equal(db.StatusPending))
		})
	})
})
