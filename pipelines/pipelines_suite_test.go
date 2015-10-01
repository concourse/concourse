package git_pipeline_test

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/concourse/atc/worker"
	"github.com/mgutz/ansi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager/lagertest"

	"testing"
	"time"
)

// has ruby, curl
const guidServerRootfs = "/var/vcap/packages/bosh_deployment_resource"

// has git, curl
const gitServerRootfs = "/var/vcap/packages/git_resource"

var flyBin string

var (
	gardenClient garden.Client

	atcURL string

	pipelineName string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	flyBinPath, err := gexec.Build("github.com/concourse/fly", "-race")
	Expect(err).NotTo(HaveOccurred())

	return []byte(flyBinPath)
}, func(flyBinPath []byte) {
	flyBin = string(flyBinPath)

	// observed jobs taking ~1m30s, so set the timeout pretty high
	SetDefaultEventuallyTimeout(5 * time.Minute)

	// poll less frequently
	SetDefaultEventuallyPollingInterval(time.Second)

	logger := lagertest.NewTestLogger("testflight")

	gardenClient = client.New(worker.RetryableConnection{
		Connection:  connection.New("tcp", "10.244.15.2:7777"),
		Logger:      logger.Session("garden-client"),
		Sleeper:     clock.NewClock(),
		RetryPolicy: worker.ExponentialRetryPolicy{Timeout: 5 * time.Minute},
	})
	Eventually(gardenClient.Ping).ShouldNot(HaveOccurred())

	atcURL = "http://10.244.15.2:8080"

	Eventually(errorPolling(atcURL)).ShouldNot(HaveOccurred())

	pipelineName = fmt.Sprintf("test-pipeline-%d", GinkgoParallelNode())
})

func TestGitPipeline(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pipelines Suite")
}

func destroyPipeline() {
	destroyCmd := exec.Command(
		flyBin,
		"-t", atcURL,
		"destroy-pipeline",
		pipelineName,
	)

	stdin, err := destroyCmd.StdinPipe()
	Expect(err).NotTo(HaveOccurred())

	defer stdin.Close()

	destroy, err := gexec.Start(destroyCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(destroy).Should(gbytes.Say("are you sure?"))

	fmt.Fprintln(stdin, "y")

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
		"-t", atcURL,
		"configure",
		pipelineName,
		"--paused=false",
	}, argv...)

	configureCmd := exec.Command(flyBin, args...)

	stdin, err := configureCmd.StdinPipe()
	Expect(err).NotTo(HaveOccurred())

	defer stdin.Close()

	configure, err := gexec.Start(configureCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(configure).Should(gbytes.Say("apply configuration?"))

	fmt.Fprintln(stdin, "y")

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
		"-t", atcURL,
		"watch",
		"-p", pipelineName,
		"-j", jobName,
	}

	if len(buildName) > 0 {
		args = append(args, "-b", buildName[0])
	}

	for {
		session := start(exec.Command(flyBin, args...))

		<-session.Exited

		if session.ExitCode() == 1 {
			output := strings.TrimSpace(string(session.Err.Contents()))
			if output == "job has no builds" || output == "build not found" {
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
