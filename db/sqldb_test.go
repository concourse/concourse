package db_test

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/BurntSushi/migration"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"

	. "github.com/concourse/atc/db"
	"github.com/concourse/atc/db/migrations"
	"github.com/concourse/atc/postgresrunner"
)

var _ = Describe("SQL DB", func() {
	var postgresPort int
	var dbConn *sql.DB

	var dbProcess ifrit.Process
	var dbDir string

	BeforeSuite(func() {
		postgresPort = 5433 + GinkgoParallelNode()

		dbProcess = ifrit.Envoke(postgresrunner.Runner{
			Addr: fmt.Sprintf("127.0.0.1:%d", postgresPort),
		})
	})

	AfterSuite(func() {
		dbProcess.Signal(os.Interrupt)
		Eventually(dbProcess.Wait()).Should(Receive())
	})

	BeforeEach(func() {
		var err error

		createdb := exec.Command("createdb", "-U", "postgres", "-p", fmt.Sprintf("%d", postgresPort), "testdb")
		createS, err := gexec.Start(createdb, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())
		Eventually(createS, 10*time.Second).Should(gexec.Exit(0))

		dbDir, err = ioutil.TempDir("", "dbDir")
		Ω(err).ShouldNot(HaveOccurred())

		err = exec.Command("cp", "-a", "../db/", dbDir+"/").Run()
		Ω(err).ShouldNot(HaveOccurred())

		dbstring := fmt.Sprintf("user=postgres dbname=testdb sslmode=disable port=%d", postgresPort)

		dbConn, err = migration.Open("postgres", dbstring, migrations.Migrations)
		Ω(err).ShouldNot(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(dbDir, "dbconf.yml"), []byte(fmt.Sprintf(`development:
  driver: postgres
  open: `+dbstring)), 0644)
		Ω(err).ShouldNot(HaveOccurred())

		db = NewSQL(dbConn)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Ω(err).ShouldNot(HaveOccurred())

		dropdb := exec.Command("dropdb", "-U", "postgres", "-p", fmt.Sprintf("%d", postgresPort), "testdb")
		dropS, err := gexec.Start(dropdb, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())
		Eventually(dropS, 10*time.Second).Should(gexec.Exit(0))

		err = os.RemoveAll(dbDir)
		Ω(err).ShouldNot(HaveOccurred())
	})

	itIsADB()
})
