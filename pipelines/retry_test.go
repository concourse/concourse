package pipelines_test

import (
	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a step that retries", func() {
	var guidServer *guidserver.Server
	var originGitServer *gitserver.Server

	BeforeEach(func() {
		guidServer = guidserver.Start(guidServerRootfs, gardenClient)
		originGitServer = gitserver.Start(gitServerRootfs, gardenClient)

		configurePipeline(
			"-c", "fixtures/retry.yml",
			"-v", "testflight-helper-image="+guidServerRootfs,
			"-v", "guid-server-register-command="+guidServer.RegisterCommand(),
			"-v", "guid-server-registrations-command="+guidServer.RegistrationsCommand(),
			"-v", "origin-git-server="+originGitServer.URI(),
		)

		originGitServer.Commit()
	})

	AfterEach(func() {
		originGitServer.Stop()
		guidServer.Stop()
	})

	It("retries until the step succeeds", func() {
		watch := flyWatch("retry-job")
		Expect(watch).To(gexec.Exit(0))

		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("registrations: 1; failing"))
		Expect(watch).To(gbytes.Say("registrations: 2; failing"))
		Expect(watch).To(gbytes.Say("registrations: 3; success!"))
		Expect(watch).ToNot(gbytes.Say("registrations:"))
		Expect(watch).To(gbytes.Say("succeeded"))
	})
})
