package pipelines_test

import (
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("serial groups", func() {
	var guidServer *guidserver.Server
	var originGitServer *gitserver.Server

	BeforeEach(func() {
		guidServer = guidserver.Start(guidServerRootfs, gardenClient)
		originGitServer = gitserver.Start(gitServerRootfs, gardenClient)

		configurePipeline(
			"-c", "fixtures/serial-groups.yml",
			"-v", "testflight-helper-image="+guidServerRootfs,
			"-v", "guid-server-curl-command="+guidServer.RegisterCommand(),
			"-v", "origin-git-server="+originGitServer.URI(),
		)
	})

	AfterEach(func() {
		guidServer.Stop()
		originGitServer.Stop()
	})

	It("runs even when another job in the serial group has a pending build", func() {
		Skip("waiting for fix in #105146972")
		pendingBuild, err := client.CreateJobBuild(pipelineName, "some-pending-job")
		Expect(err).NotTo(HaveOccurred())

		guid1 := originGitServer.Commit()
		Eventually(guidServer.ReportingGuids).Should(ContainElement(guid1))

		updatedPendingBuild, found, err := client.Build(fmt.Sprint(pendingBuild.ID))
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(updatedPendingBuild.Status).To(Equal(string(atc.StatusPending)))
	})
})
