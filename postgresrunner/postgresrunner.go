package postgresrunner

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"strconv"

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
	defer ginkgo.GinkgoRecover()

	tmpdir, err := ioutil.TempDir("", "postgres")
	Expect(err).NotTo(HaveOccurred())

	currentUser, err := user.Current()
	Expect(err).NotTo(HaveOccurred())

	var initCmd, startCmd *exec.Cmd

	initdbPath, err := exec.LookPath("initdb")
	Expect(err).NotTo(HaveOccurred())

	postgresPath, err := exec.LookPath("postgres")
	Expect(err).NotTo(HaveOccurred())

	initdb := initdbPath + " -U postgres -D " + tmpdir
	postgres := fmt.Sprintf("%s -d 2 -D %s -h 127.0.0.1 -p %d", postgresPath, tmpdir, runner.Port)

	if currentUser.Uid == "0" {
		pgUser, err := user.Lookup("postgres")
		Expect(err).NotTo(HaveOccurred())

		uid, err := strconv.Atoi(pgUser.Uid)
		Expect(err).NotTo(HaveOccurred())

		gid, err := strconv.Atoi(pgUser.Gid)
		Expect(err).NotTo(HaveOccurred())

		err = os.Chown(tmpdir, uid, gid)
		Expect(err).NotTo(HaveOccurred())

		initCmd = exec.Command("su", "postgres", "-c", initdb)
		startCmd = exec.Command("su", "postgres", "-c", postgres)
	} else {
		initCmd = exec.Command("bash", "-c", initdb)
		startCmd = exec.Command("bash", "-c", postgres)
	}

	session, err := gexec.Start(
		initCmd,
		gexec.NewPrefixedWriter("[o][initdb] ", ginkgo.GinkgoWriter),
		gexec.NewPrefixedWriter("[e][initdb] ", ginkgo.GinkgoWriter),
	)
	Expect(err).NotTo(HaveOccurred())

	<-session.Exited

	Expect(session).To(gexec.Exit(0))

	ginkgoRunner := &ginkgomon.Runner{
		Name:          "postgres",
		Command:       startCmd,
		AnsiColorCode: "90m",
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
		runner.DataSourceName(),
		migrations.Migrations,
	)
	Expect(err).NotTo(HaveOccurred())

	return dbConn
}

func (runner *Runner) DataSourceName() string {
	return fmt.Sprintf("user=postgres dbname=testdb sslmode=disable port=%d", runner.Port)
}

func (runner *Runner) CreateTestDB() {
	createdb := exec.Command("createdb", "-U", "postgres", "-p", strconv.Itoa(runner.Port), "testdb")

	createS, err := gexec.Start(createdb, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-createS.Exited

	if createS.ExitCode() != 0 {
		runner.DropTestDB()

		createdb := exec.Command("createdb", "-U", "postgres", "-p", strconv.Itoa(runner.Port), "testdb")
		createS, err = gexec.Start(createdb, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
	}

	<-createS.Exited

	Expect(createS).To(gexec.Exit(0))
}

func (runner *Runner) DropTestDB() {
	dropdb := exec.Command("dropdb", "-U", "postgres", "-p", strconv.Itoa(runner.Port), "testdb")
	dropS, err := gexec.Start(dropdb, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-dropS.Exited

	Expect(dropS).To(gexec.Exit(0))
}
