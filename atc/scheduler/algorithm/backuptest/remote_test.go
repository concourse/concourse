package algorithm_test

import (
	"database/sql"
	"log"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/scheduler/algorithm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resolve", func() {
	var teamFactory db.TeamFactory
	var inputMapper algorithm.InputMapper

	BeforeSuite(func() {
		driverName := "postgres"
		connString := "host=/var/run/postgresql dbname=alg"

		logger := lagertest.NewTestLogger("test")

		lockConn, err := sql.Open(driverName, connString)
		Expect(err).NotTo(HaveOccurred())

		lockConn.SetMaxOpenConns(1)
		lockConn.SetMaxIdleConns(1)
		lockConn.SetConnMaxLifetime(0)

		lockFactory := lock.NewLockFactory(lockConn, metric.LogLockAcquired, metric.LogLockReleased)

		dbConn, err := db.Open(logger, driverName, connString, nil, nil, "test", lockFactory)
		Expect(err).NotTo(HaveOccurred())

		dbConn = db.Log(logger.Session("log-conn"), dbConn)

		teamFactory = db.NewTeamFactory(dbConn, lockFactory)

		inputMapper = algorithm.NewInputMapper()
	})

	FIt("schedules all jobs", func() {
		teams, err := teamFactory.GetTeams()
		Expect(err).NotTo(HaveOccurred())

		for _, t := range teams {
			pipelines, err := t.Pipelines()
			Expect(err).NotTo(HaveOccurred())

			for _, p := range pipelines {
				versionsDB, err := p.LoadVersionsDB()
				Expect(err).NotTo(HaveOccurred())

				resources, err := p.Resources()
				Expect(err).NotTo(HaveOccurred())

				jobs, err := p.Jobs()
				Expect(err).NotTo(HaveOccurred())

				for _, j := range jobs {
					log.Println("scheduling", t.Name(), p.Name(), j.Name())

					inputMapping, ok, err := inputMapper.MapInputs(versionsDB, j, resources)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(BeTrue())

					log.Printf("inputs: %#v\n", inputMapping)
				}
			}
		}
	})
})
