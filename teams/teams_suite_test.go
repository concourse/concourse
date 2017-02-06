package teams_test

import (
	"fmt"
	"os"
	"os/exec"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/go-concourse/concourse"
	"github.com/concourse/testflight/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	client concourse.Client
	team   concourse.Team

	flyBin string

	tmpHome string
	logger  lager.Logger
)

var atcURL = helpers.AtcURL()
var targetedConcourse = "testflight"

var _ = SynchronizedBeforeSuite(func() []byte {
	Eventually(helpers.ErrorPolling(atcURL)).ShouldNot(HaveOccurred())

	data, err := helpers.FirstNodeFlySetup(atcURL, targetedConcourse)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	var err error
	flyBin, tmpHome, err = helpers.AllNodeFlySetup(data)
	Expect(err).NotTo(HaveOccurred())

	client, err = helpers.AllNodeClientSetup(data)
	Expect(err).NotTo(HaveOccurred())

	team = client.Team("main")
	logger = lagertest.NewTestLogger("teams-test")
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	os.RemoveAll(tmpHome)
})

func TestTeams(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Teams Suite")
}

func setTeam(name string) {
	args := []string{
		"-t", targetedConcourse,
		"set-team",
		"-n", name,
		"--no-really-i-dont-want-any-auth",
	}

	setTeamCmd := exec.Command(flyBin, args...)

	setTeamIn, err := setTeamCmd.StdinPipe()
	Expect(err).NotTo(HaveOccurred())

	setTeam, err := gexec.Start(setTeamCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	buffer := setTeam.Buffer()

	Eventually(func() string {
		return string(buffer.Contents())
	}).Should(ContainSubstring("apply configuration? [yN]:"))

	fmt.Fprintln(setTeamIn, "y")
	Eventually(setTeam).Should(gexec.Exit(0))
}

func destroyTeam(name string) {
	args := []string{
		"-t", targetedConcourse,
		"destroy-team",
		"-n", name,
	}

	destroyTeamCmd := exec.Command(flyBin, args...)

	destroyTeamIn, err := destroyTeamCmd.StdinPipe()
	Expect(err).NotTo(HaveOccurred())

	destroyTeam, err := gexec.Start(destroyTeamCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	buffer := destroyTeam.Buffer()

	Eventually(func() string {
		return string(buffer.Contents())
	}).Should(ContainSubstring("please type the team name to confirm:"))

	fmt.Fprintln(destroyTeamIn, name)
	Eventually(destroyTeam).Should(gexec.Exit())
}

func renameTeam(oldName string, newName string) {
	args := []string{
		"-t", targetedConcourse,
		"rename-team",
		"-o", oldName,
		"-n", newName,
	}

	renameTeamCmd := exec.Command(flyBin, args...)

	renameTeam, err := gexec.Start(renameTeamCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-renameTeam.Exited
	Expect(renameTeam).To(gexec.Exit(0))
}

func listTeams() []byte {
	args := []string{
		"-t", targetedConcourse,
		"teams",
	}

	listTeamsCmd := exec.Command(flyBin, args...)

	listTeams, err := gexec.Start(listTeamsCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-listTeams.Exited
	Expect(listTeams).To(gexec.Exit(0))
	return listTeams.Out.Contents()
}
