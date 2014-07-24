package postgresrunner

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"time"

	"github.com/BurntSushi/migration"
	"github.com/concourse/atc/db/migrations"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit/ginkgomon"
)

type Runner struct {
	Port int
}

func (runner Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	tmpdir, err := ioutil.TempDir("", "postgres")
	Ω(err).ShouldNot(HaveOccurred())

	currentUser, err := user.Current()
	Ω(err).ShouldNot(HaveOccurred())

	var initCmd, startCmd *exec.Cmd

	initdbPath, err := exec.LookPath("initdb")
	Ω(err).ShouldNot(HaveOccurred())

	postgresPath, err := exec.LookPath("postgres")
	Ω(err).ShouldNot(HaveOccurred())

	initdb := initdbPath + " -U postgres -D " + tmpdir
	postgres := fmt.Sprintf("%s -d 2 -D %s -h 127.0.0.1 -p %d", postgresPath, tmpdir, runner.Port)

	if currentUser.Uid == "0" {
		pgUser, err := user.Lookup("postgres")
		Ω(err).ShouldNot(HaveOccurred())

		uid, err := strconv.Atoi(pgUser.Uid)
		Ω(err).ShouldNot(HaveOccurred())

		gid, err := strconv.Atoi(pgUser.Gid)
		Ω(err).ShouldNot(HaveOccurred())

		err = os.Chown(tmpdir, uid, gid)
		Ω(err).ShouldNot(HaveOccurred())

		initCmd = exec.Command("su", "postgres", "-c", initdb)
		startCmd = exec.Command("su", "postgres", "-c", postgres)
	} else {
		initCmd = exec.Command("bash", "-c", initdb)
		startCmd = exec.Command("bash", "-c", postgres)
	}

	session, err := gexec.Start(
		initCmd,
		gexec.NewPrefixedWriter("[out][initdb] ", ginkgo.GinkgoWriter),
		gexec.NewPrefixedWriter("[err][initdb] ", ginkgo.GinkgoWriter),
	)
	Ω(err).ShouldNot(HaveOccurred())

	Eventually(session, 60*time.Second).Should(gexec.Exit(0))

	ginkgoRunner := &ginkgomon.Runner{
		Name:          "postgres",
		Command:       startCmd,
		AnsiColorCode: "91",
		StartCheck:    "database system is ready to accept connections",
		Cleanup: func() {
			os.RemoveAll(tmpdir)
		},
	}

	return ginkgoRunner.Run(signals, ready)
}

func (runner *Runner) Open() *sql.DB {
	dbConn, err := migration.Open(
		"postgres",
		fmt.Sprintf("user=postgres dbname=testdb sslmode=disable port=%d", runner.Port),
		migrations.Migrations,
	)
	Ω(err).ShouldNot(HaveOccurred())

	return dbConn
}

func (runner *Runner) CreateTestDB() {
	createdb := exec.Command("createdb", "-U", "postgres", "-p", strconv.Itoa(runner.Port), "testdb")
	createS, err := gexec.Start(createdb, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	Ω(err).ShouldNot(HaveOccurred())
	Eventually(createS, 10*time.Second).Should(gexec.Exit(0))
}

func (runner *Runner) DropTestDB() {
	dropdb := exec.Command("dropdb", "-U", "postgres", "-p", strconv.Itoa(runner.Port), "testdb")
	dropS, err := gexec.Start(dropdb, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	Ω(err).ShouldNot(HaveOccurred())
	Eventually(dropS, 10*time.Second).Should(gexec.Exit(0))
}
