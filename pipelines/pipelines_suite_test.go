package pipelines_test

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/concourse/testflight/helpers"
	"github.com/mgutz/ansi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-golang/lager/lagertest"

	gconn "github.com/cloudfoundry-incubator/garden/client/connection"

	"testing"
	"time"
)

// has ruby, curl
const guidServerRootfs = "/var/vcap/packages/bosh_deployment_resource"

// has git, curl
const gitServerRootfs = "/var/vcap/packages/git_resource"

var (
	gardenClient garden.Client

	flyBin string

	pipelineName string

	tmpHome string
)

var atcURL = "http://10.244.15.2:8080"
var targetedConcourse = "testflight"

var _ = SynchronizedBeforeSuite(func() []byte {
	flyBinPath, err := gexec.Build("github.com/concourse/fly", "-race")
	Expect(err).NotTo(HaveOccurred())

	return []byte(flyBinPath)
}, func(flyBinPath []byte) {
	flyBin = string(flyBinPath)

	var err error
	tmpHome, err = helpers.CreateTempHomeDir()
	Expect(err).NotTo(HaveOccurred())

	err = helpers.FlyLogin(atcURL, targetedConcourse, flyBin)
	Expect(err).NotTo(HaveOccurred())

	// observed jobs taking ~1m30s, so set the timeout pretty high
	SetDefaultEventuallyTimeout(5 * time.Minute)

	// poll less frequently
	SetDefaultEventuallyPollingInterval(time.Second)

	logger := lagertest.NewTestLogger("testflight")

	gardenClient = client.New(gconn.NewWithLogger("tcp", "10.244.15.2:7777", logger.Session("garden-connection")))
	Eventually(gardenClient.Ping).ShouldNot(HaveOccurred())

	Eventually(errorPolling(atcURL)).ShouldNot(HaveOccurred())

	pipelineName = fmt.Sprintf("test-pipeline-%d", GinkgoParallelNode())
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	os.RemoveAll(tmpHome)
})

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
	)

	stdin, err := destroyCmd.StdinPipe()
	Expect(err).NotTo(HaveOccurred())

	defer stdin.Close()

	destroy, err := gexec.Start(destroyCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(destroy).Should(gbytes.Say("are you sure?"))

	fmt.Fprintf(stdin, "y\n")

	<-destroy.Exited

	if destroy.ExitCode() == 1 {
		if strings.Contains(string(destroy.Err.Contents()), "does not exist") {
			return
		}
	}

	Expect(destroy).To(gexec.Exit(0))
}

func configurePipeline(argv ...string) {
	destroyPipeline()

	args := append([]string{
		"-t", targetedConcourse,
		"set-pipeline",
		"-p", pipelineName,
	}, argv...)

	configureCmd := exec.Command(flyBin, args...)

	stdin, err := configureCmd.StdinPipe()
	Expect(err).NotTo(HaveOccurred())

	defer stdin.Close()

	configure, err := gexec.Start(configureCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(configure).Should(gbytes.Say("apply configuration?"))

	fmt.Fprintf(stdin, "y\n")

	Eventually(configure).Should(gexec.Exit(0))
	unpausePipeline()
}

func unpausePipeline() {
	unpauseCmd := exec.Command(flyBin, "-t", targetedConcourse, "unpause-pipeline", "-p", pipelineName)

	stdin, err := unpauseCmd.StdinPipe()
	Expect(err).NotTo(HaveOccurred())

	defer stdin.Close()

	configure, err := gexec.Start(unpauseCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(configure).Should(gbytes.Say("unpaused '%s'", pipelineName))
	Eventually(configure).Should(gexec.Exit(0))
}

func errorPolling(url string) func() error {
	return func() error {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
		}

		return err
	}
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
