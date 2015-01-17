package git_pipeline_test

import (
	"os"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/concourse/testflight/bosh"
	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
	"time"
)

const helperRootfs = "docker:///concourse/testflight-helper"

var (
	gardenClient garden.Client

	gitServer *gitserver.Server

	successGitServer  *gitserver.Server
	failureGitServer  *gitserver.Server
	noUpdateGitServer *gitserver.Server
)

type GardenLinuxDeploymentData struct {
	GardenLinuxVersion string
}

type GitPipelineTemplate struct {
	GitServers struct {
		Origin   string
		Success  string
		Failure  string
		NoUpdate string
	}

	GuidServerCurlCommand string

	TestflightHelperImage string

	GardenLinuxDeploymentData
}

var _ = BeforeSuite(func() {
	gardenLinuxVersion := os.Getenv("GARDEN_LINUX_VERSION")
	Ω(gardenLinuxVersion).ShouldNot(BeEmpty(), "must set $GARDEN_LINUX_VERSION")

	bosh.DeleteDeployment("garden-testflight")
	bosh.DeleteDeployment("concourse-testflight")

	gardenLinuxDeploymentData := GardenLinuxDeploymentData{
		GardenLinuxVersion: gardenLinuxVersion,
	}

	bosh.Deploy("garden.yml", gardenLinuxDeploymentData)

	gardenClient = client.New(connection.New("tcp", "10.244.16.2:7777"))
	Eventually(gardenClient.Ping, 10*time.Second).ShouldNot(HaveOccurred())

	guidserver.Start(helperRootfs, gardenClient)

	gitServer = gitserver.Start(helperRootfs, gardenClient)
	successGitServer = gitserver.Start(helperRootfs, gardenClient)
	failureGitServer = gitserver.Start(helperRootfs, gardenClient)
	noUpdateGitServer = gitserver.Start(helperRootfs, gardenClient)

	templateData := GitPipelineTemplate{
		GardenLinuxDeploymentData: gardenLinuxDeploymentData,
	}

	templateData.GitServers.Origin = gitServer.URI()
	templateData.GitServers.Success = successGitServer.URI()
	templateData.GitServers.Failure = failureGitServer.URI()
	templateData.GitServers.NoUpdate = noUpdateGitServer.URI()

	templateData.TestflightHelperImage = helperRootfs
	templateData.GuidServerCurlCommand = guidserver.CurlCommand()

	bosh.Deploy("deployment.yml.tmpl", templateData)
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
