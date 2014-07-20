package postgresrunner

import (
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"time"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit/ginkgomon"
)

type Runner struct {
	Addr string
}

func (runner Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	tmpdir, err := ioutil.TempDir("", "postgres")
	Ω(err).ShouldNot(HaveOccurred())

	host, port, err := net.SplitHostPort(runner.Addr)
	Ω(err).ShouldNot(HaveOccurred())

	currentUser, err := user.Current()
	Ω(err).ShouldNot(HaveOccurred())

	var initCmd, startCmd *exec.Cmd

	initdbPath, err := exec.LookPath("initdb")
	Ω(err).ShouldNot(HaveOccurred())

	postgresPath, err := exec.LookPath("postgres")
	Ω(err).ShouldNot(HaveOccurred())

	initdb := initdbPath + " -U postgres -D " + tmpdir
	postgres := postgresPath + " -d 2 -D " + tmpdir + " -h " + host + " -p " + port

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
	}

	return ginkgoRunner.Run(signals, ready)
}
