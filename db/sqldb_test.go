package db_test

import (
	"database/sql"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"

	. "github.com/concourse/atc/db"
	"github.com/concourse/atc/postgresrunner"
)

var _ = Describe("SQL DB", func() {
	var postgresRunner postgresrunner.Runner

	var dbConn *sql.DB
	var dbProcess ifrit.Process

	BeforeSuite(func() {
		postgresRunner = postgresrunner.Runner{
			Port: 5433 + GinkgoParallelNode(),
		}

		dbProcess = ifrit.Envoke(postgresRunner)
	})

	AfterSuite(func() {
		dbProcess.Signal(os.Interrupt)
		Eventually(dbProcess.Wait(), 10*time.Second).Should(Receive())
	})

	BeforeEach(func() {
		postgresRunner.CreateTestDB()

		dbConn = postgresRunner.Open()
		db = NewSQL(lagertest.NewTestLogger("test"), dbConn)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Ω(err).ShouldNot(HaveOccurred())

		postgresRunner.DropTestDB()
	})

	itIsADB()
})
