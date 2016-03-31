package pipelines_test

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/go-concourse/concourse"
	"github.com/concourse/testflight/helpers"
	"github.com/mgutz/ansi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-golang/lager/lagertest"

	gclient "github.com/cloudfoundry-incubator/garden/client"
	gconn "github.com/cloudfoundry-incubator/garden/client/connection"

	"testing"
	"time"
)

var (
	client concourse.Client

	gardenClient garden.Client

	flyBin string

	pipelineName string

	tmpHome string

	// needs ruby, curl
	guidServerRootfs string

	// needss git, curl
	gitServerRootfs string
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

	logger := lagertest.NewTestLogger("testflight")

	workers, err := client.ListWorkers()
	Expect(err).NotTo(HaveOccurred())

	gLog := logger.Session("garden-connection")

	for _, w := range workers {
		gitServerRootfs = ""
		guidServerRootfs = ""

		for _, r := range w.ResourceTypes {
			if r.Type == "git" {
				gitServerRootfs = r.Image
			} else if r.Type == "bosh-deployment" {
				guidServerRootfs = r.Image
			}
		}

		if gitServerRootfs != "" && guidServerRootfs != "" {
			gardenClient = gclient.New(gconn.NewWithLogger("tcp", w.GardenAddr, gLog))
		}
	}

	if gitServerRootfs == "" || guidServerRootfs == "" {
		Fail("must have at least one worker that supports git and bosh-deployment resource types")
	}

	Eventually(gardenClient.Ping).Should(Succeed())

	pipelineName = fmt.Sprintf("test-pipeline-%d", GinkgoParallelNode())
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	os.RemoveAll(tmpHome)
})

var _ = AfterEach(destroyPipeline)

func TestGitPipeline(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pipelines Suite")
}

func destroyPipeline() {
	destroyCmd := exec.Command(
		flyBin,
		"-t", targetedConcourse,
		"destroy-pipeline",
		"-p", pipelineName,
		"-n",
	)

	destroy, err := gexec.Start(destroyCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-destroy.Exited

	if destroy.ExitCode() == 1 {
		if strings.Contains(string(destroy.Err.Contents()), "does not exist") {
			return
		}
	}

	Expect(destroy).To(gexec.Exit(0))
}

func renamePipeline(newName string) {
	renameCmd := exec.Command(
		flyBin,
		"-t", targetedConcourse,
		"rename-pipeline",
		"-o", pipelineName,
		"-n", newName,
	)

	rename, err := gexec.Start(renameCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-rename.Exited
	Expect(rename).To(gexec.Exit(0))

	pipelineName = newName
}

func configurePipeline(argv ...string) {
	destroyPipeline()

	reconfigurePipeline(argv...)
}

func reconfigurePipeline(argv ...string) {
	args := append([]string{
		"-t", targetedConcourse,
		"set-pipeline",
		"-p", pipelineName,
		"-n",
	}, argv...)

	configureCmd := exec.Command(flyBin, args...)

	configure, err := gexec.Start(configureCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-configure.Exited
	Expect(configure.ExitCode()).To(Equal(0))

	unpausePipeline()
}

func unpausePipeline() {
	unpauseCmd := exec.Command(flyBin, "-t", targetedConcourse, "unpause-pipeline", "-p", pipelineName)

	configure, err := gexec.Start(unpauseCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-configure.Exited
	Expect(configure.ExitCode()).To(Equal(0))

	Expect(configure).To(gbytes.Say("unpaused '%s'", pipelineName))
}

func flyWatch(jobName string, buildName ...string) *gexec.Session {
	args := []string{
		"-t", targetedConcourse,
		"watch",
		"-j", pipelineName + "/" + jobName,
	}

	if len(buildName) > 0 {
		args = append(args, "-b", buildName[0])
	}

	keepPollingCheck := regexp.MustCompile("job has no builds|build not found|failed to get build")
	for {
		session := start(exec.Command(flyBin, args...))

		<-session.Exited

		if session.ExitCode() == 1 {
			output := strings.TrimSpace(string(session.Err.Contents()))
			if keepPollingCheck.MatchString(output) {
				// build hasn't started yet; keep polling
				time.Sleep(time.Second)
				continue
			}
		}

		return session
	}
}

func start(cmd *exec.Cmd) *gexec.Session {
	session, err := gexec.Start(
		cmd,
		gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[o]", "green"), ansi.Color("[fly]", "blue")),
			GinkgoWriter,
		),
		gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[e]", "red+bright"), ansi.Color("[fly]", "blue")),
			GinkgoWriter,
		),
	)
	Expect(err).NotTo(HaveOccurred())

	return session
}
