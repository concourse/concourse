package db_test

import (
	"time"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/dbng"
)

var _ = Describe("SQL DB Teams", func() {
	var dbConn db.Conn
	var dbngConn dbng.Conn
	var listener *pq.Listener

	var database *db.SQLDB
	var workerFactory dbng.WorkerFactory
	var teamDBFactory db.TeamDBFactory

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.OpenDB())
		dbngConn = postgresRunner.OpenConn()
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		lockFactory := lock.NewLockFactory(postgresRunner.OpenSingleton())
		teamDBFactory = db.NewTeamDBFactory(dbConn, bus, lockFactory)
		database = db.NewSQL(dbConn, bus, lockFactory)

		workerFactory = dbng.NewWorkerFactory(dbngConn)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("CreateTeam", func() {
		It("saves a team to the db", func() {
			expectedTeam := db.Team{
				Name: "AvengerS",
			}
			expectedSavedTeam, err := database.CreateTeam(expectedTeam)
			Expect(err).NotTo(HaveOccurred())
			Expect(expectedSavedTeam.Team.Admin).To(Equal(expectedTeam.Admin))
			Expect(expectedSavedTeam.Team.Name).To(Equal("AvengerS"))

			savedTeam, found, err := teamDBFactory.GetTeamDB("aVengers").GetTeam()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(savedTeam).To(Equal(expectedSavedTeam))
		})
	})
})
