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

const helperRootfs = "docker:///concourse/testflight-helper"

var flyBin string

var (
	gardenClient garden.Client

	gitServer *gitserver.Server

	successGitServer  *gitserver.Server
	failureGitServer  *gitserver.Server
	noUpdateGitServer *gitserver.Server
)

type GardenLinuxDeploymentData struct {
	DirectorUUID string

	GardenLinuxVersion string
}

type GitPipelineTemplate struct {
	DirectorUUID string
	GardenLinuxDeploymentData
}

var _ = BeforeSuite(func() {
	gardenLinuxVersion := os.Getenv("GARDEN_LINUX_VERSION")
	Ω(gardenLinuxVersion).ShouldNot(BeEmpty(), "must set $GARDEN_LINUX_VERSION")

	var err error

	flyBin, err = gexec.Build("github.com/concourse/fly", "-race")
	Ω(err).ShouldNot(HaveOccurred())

	directorUUID := bosh.DirectorUUID()

	bosh.DeleteDeployment("garden-testflight")
	bosh.DeleteDeployment("concourse-testflight")

	gardenLinuxDeploymentData := GardenLinuxDeploymentData{
		DirectorUUID:       directorUUID,
		GardenLinuxVersion: gardenLinuxVersion,
	}

	bosh.Deploy("garden.yml.tmpl", gardenLinuxDeploymentData)

	gardenClient = client.New(connection.New("tcp", "10.244.16.2:7777"))
	Eventually(gardenClient.Ping, 10*time.Second).ShouldNot(HaveOccurred())

	guidserver.Start(helperRootfs, gardenClient)

	gitServer = gitserver.Start(helperRootfs, gardenClient)
	successGitServer = gitserver.Start(helperRootfs, gardenClient)
	failureGitServer = gitserver.Start(helperRootfs, gardenClient)
	noUpdateGitServer = gitserver.Start(helperRootfs, gardenClient)

	templateData := GitPipelineTemplate{
		DirectorUUID:              directorUUID,
		GardenLinuxDeploymentData: gardenLinuxDeploymentData,
	}

	bosh.Deploy("deployment.yml.tmpl", templateData)

	atcURL := "http://10.244.15.2:8080"

	Eventually(errorPolling(atcURL), 1*time.Minute).ShouldNot(HaveOccurred())

	configureCmd := exec.Command(
		flyBin,
		"-t", atcURL,
		"configure",
		"-c", "pipeline.yml",
		"-v", "failure-git-server="+failureGitServer.URI(),
		"-v", "guid-server-curl-command="+guidserver.CurlCommand(),
		"-v", "no-update-git-server="+noUpdateGitServer.URI(),
		"-v", "origin-git-server="+gitServer.URI(),
		"-v", "success-git-server="+successGitServer.URI(),
		"-v", "testflight-helper-image="+helperRootfs,
	)

	stdin, err := configureCmd.StdinPipe()
	Ω(err).ShouldNot(HaveOccurred())

	defer stdin.Close()

	configure, err := gexec.Start(configureCmd, GinkgoWriter, GinkgoWriter)
	Ω(err).ShouldNot(HaveOccurred())

	Eventually(configure, 10).Should(gbytes.Say("apply configuration?"))

	fmt.Fprintln(stdin, "y")

	Eventually(configure, 10).Should(gexec.Exit(0))
})

var _ = AfterSuite(func() {
	gitServer.Stop()
	successGitServer.Stop()
	failureGitServer.Stop()
	noUpdateGitServer.Stop()

	guidserver.Stop(gardenClient)
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
