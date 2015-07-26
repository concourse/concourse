package git_pipeline_test

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/mgutz/ansi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

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

var _ = BeforeSuite(func() {
	// observed jobs taking ~1m30s, so set the timeout pretty high
	SetDefaultEventuallyTimeout(5 * time.Minute)

	// poll less frequently
	SetDefaultEventuallyPollingInterval(time.Second)

	var err error

	flyBin, err = gexec.Build("github.com/concourse/fly", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	gardenClient = client.New(connection.New("tcp", "10.244.15.2:7777"))
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
	destroyCmd := exec.Command("fly", "destroy-pipeline", pipelineName)

	stdin, err := destroyCmd.StdinPipe()
	Ω(err).ShouldNot(HaveOccurred())

	defer stdin.Close()

	destroy, err := gexec.Start(destroyCmd, GinkgoWriter, GinkgoWriter)
	Ω(err).ShouldNot(HaveOccurred())

	Eventually(destroy).Should(gbytes.Say("are you sure?"))

	fmt.Fprintln(stdin, "y")

	Eventually(destroy).Should(gexec.Exit(0))
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
	Ω(err).ShouldNot(HaveOccurred())

	defer stdin.Close()

	configure, err := gexec.Start(configureCmd, GinkgoWriter, GinkgoWriter)
	Ω(err).ShouldNot(HaveOccurred())

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

func flyWatch(jobName string) *gexec.Session {
	for {
		session := start(exec.Command(
			flyBin,
			"-t", atcURL,
			"watch",
			"-p", pipelineName,
			"-j", jobName,
		))

		<-session.Exited

		if session.ExitCode() == 1 {
			output := strings.TrimSpace(string(session.Err.Contents()))
			if output == "job has no builds" {
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
	Ω(err).ShouldNot(HaveOccurred())

	return session
}
