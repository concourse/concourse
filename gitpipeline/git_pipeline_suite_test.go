package git_pipeline_test

import (
	"os"

	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/cloudfoundry-incubator/garden/warden"
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
	gardenClient warden.Client

	gitServer *gitserver.Server

	successGitServer  *gitserver.Server
	failureGitServer  *gitserver.Server
	noUpdateGitServer *gitserver.Server
)

type GitPipelineTemplate struct {
	GitServers struct {
		Origin   string
		Success  string
		Failure  string
		NoUpdate string
	}

	GuidServerCurlCommand string

	TestflightHelperImage string
}

var _ = BeforeEach(func() {
	Ω(os.Getenv("BOSH_LITE_IP")).ShouldNot(BeEmpty(), "must specify $BOSH_LITE_IP")

	bosh.DeployConcourse("noop.yml")

	gardenClient = client.New(connection.New("tcp", os.Getenv("BOSH_LITE_IP")+":7777"))

	Eventually(gardenClient.Ping, 10*time.Second).ShouldNot(HaveOccurred())

	guidserver.Start(helperRootfs, gardenClient)

	gitServer = gitserver.Start(helperRootfs, gardenClient)
	gitServer.Commit()

	successGitServer = gitserver.Start(helperRootfs, gardenClient)
	failureGitServer = gitserver.Start(helperRootfs, gardenClient)
	noUpdateGitServer = gitserver.Start(helperRootfs, gardenClient)

	templateData := GitPipelineTemplate{
		TestflightHelperImage: helperRootfs,
		GuidServerCurlCommand: guidserver.CurlCommand(),
	}

	templateData.GitServers.Origin = gitServer.URI()
	templateData.GitServers.Success = successGitServer.URI()
	templateData.GitServers.Failure = failureGitServer.URI()
	templateData.GitServers.NoUpdate = noUpdateGitServer.URI()

	bosh.DeployConcourse("deployment.yml.tmpl", templateData)

	Eventually(gardenClient.Ping, 10*time.Second).ShouldNot(HaveOccurred())
})

var _ = AfterEach(func() {
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
