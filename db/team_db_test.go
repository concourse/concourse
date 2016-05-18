package db_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TeamDB", func() {
	var dbConn db.Conn
	var listener *pq.Listener

	var database db.DB
	var teamDBFactory db.TeamDBFactory

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		teamDBFactory = db.NewTeamDBFactory(dbConn)
		database = db.NewSQL(dbConn, bus)

		database.DeleteTeamByName(atc.DefaultTeamName)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when the team exists", func() {
		BeforeEach(func() {
			team := db.Team{
				Name: "team-name",
			}
			_, err := database.CreateTeam(team)
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("GetTeam", func() {
			It("returns the saved team when finding by an exact match", func() {
				teamDB := teamDBFactory.GetTeamDB("team-name")
				actualTeam, found, err := teamDB.GetTeam()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(actualTeam.Name).To(Equal("team-name"))
			})

			It("returns the saved team when finding by a case-insensitive match", func() {
				teamDB := teamDBFactory.GetTeamDB("TEAM-name")
				actualTeam, found, err := teamDB.GetTeam()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(actualTeam.Name).To(Equal("team-name"))
			})
		})
	})

	Context("when the team does not exist", func() {
		var teamDB db.TeamDB

		BeforeEach(func() {
			teamDB = teamDBFactory.GetTeamDB("nonexistent-team")
		})

		Describe("GetTeam", func() {
			It("returns false with no error when the team does not exist", func() {
				_, found, err := teamDB.GetTeam()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})
})
