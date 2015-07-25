package git_pipeline_test

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/concourse/testflight/bosh"
	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
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

	gitServer *gitserver.Server

	successGitServer       *gitserver.Server
	failureGitServer       *gitserver.Server
	noUpdateGitServer      *gitserver.Server
	ensureSuccessGitServer *gitserver.Server
	ensureFailureGitServer *gitserver.Server

	atcURL string
)

type DeploymentTemplateData struct {
	DirectorUUID       string
	GardenLinuxVersion string
}

var _ = BeforeSuite(func() {
	SetDefaultEventuallyTimeout(time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	gardenLinuxVersion := os.Getenv("GARDEN_LINUX_VERSION")
	Ω(gardenLinuxVersion).ShouldNot(BeEmpty(), "must set $GARDEN_LINUX_VERSION")

	var err error

	flyBin, err = gexec.Build("github.com/concourse/fly", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	directorUUID := bosh.DirectorUUID()

	bosh.DeleteDeployment("concourse-testflight")

	deploymentData := DeploymentTemplateData{
		DirectorUUID:       directorUUID,
		GardenLinuxVersion: gardenLinuxVersion,
	}

	bosh.Deploy("deployment.yml.tmpl", deploymentData)

	gardenClient = client.New(connection.New("tcp", "10.244.15.2:7777"))
	Eventually(gardenClient.Ping).ShouldNot(HaveOccurred())

	guidserver.Start(guidServerRootfs, gardenClient)

	gitServer = gitserver.Start(gitServerRootfs, gardenClient)
	successGitServer = gitserver.Start(gitServerRootfs, gardenClient)
	failureGitServer = gitserver.Start(gitServerRootfs, gardenClient)
	noUpdateGitServer = gitserver.Start(gitServerRootfs, gardenClient)
	ensureSuccessGitServer = gitserver.Start(gitServerRootfs, gardenClient)
	ensureFailureGitServer = gitserver.Start(gitServerRootfs, gardenClient)

	atcURL = "http://10.244.15.2:8080"

	Eventually(errorPolling(atcURL)).ShouldNot(HaveOccurred())

	configureCmd := exec.Command(
		flyBin,
		"-t", atcURL,
		"configure",
		"pipeline-name",
		"-c", "pipeline.yml",
		"-v", "failure-git-server="+failureGitServer.URI(),
		"-v", "guid-server-curl-command="+guidserver.CurlCommand(),
		"-v", "no-update-git-server="+noUpdateGitServer.URI(),
		"-v", "origin-git-server="+gitServer.URI(),
		"-v", "success-git-server="+successGitServer.URI(),
		"-v", "ensure-success-git-server="+ensureSuccessGitServer.URI(),
		"-v", "ensure-failure-git-server="+ensureFailureGitServer.URI(),
		"-v", "testflight-helper-image="+guidServerRootfs,
		"--paused=false",
	)

	stdin, err := configureCmd.StdinPipe()
	Ω(err).ShouldNot(HaveOccurred())

	defer stdin.Close()

	configure, err := gexec.Start(configureCmd, GinkgoWriter, GinkgoWriter)
	Ω(err).ShouldNot(HaveOccurred())

	Eventually(configure).Should(gbytes.Say("apply configuration?"))

	fmt.Fprintln(stdin, "y")

	Eventually(configure).Should(gexec.Exit(0))
})

func TestGitPipeline(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Git Pipeline Suite")
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
