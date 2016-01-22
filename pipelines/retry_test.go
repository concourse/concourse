package pipelines_test

import (
	"os/exec"

	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
	"github.com/concourse/testflight/helpers"
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

	Context("hijacking jobs with retry steps", func() {
		var hijackS *gexec.Session
		var hijack *exec.Cmd

		BeforeEach(func() {
			// wait until job finishes before trying to hijack
			watch := flyWatch("retry-job")
			Expect(watch).To(gexec.Exit(0))
		})

		It("permits hijacking a specific attempt", func() {
			hijack = exec.Command(flyBin, "-t", targetedConcourse, "hijack",
				"-j", pipelineName+"/retry-job",
				"-s", "curl-server-until-3-curls",
				"--attempt", "2",
				"--", "sh", "-c",
				"if [ `cat /tmp/retry_number` -eq 2 ]; then exit 0; else exit 1; fi;")
			hijackS = helpers.StartFly(hijack)
			Eventually(hijackS).Should(gexec.Exit(0))
		})

		It("correctly displays information about attempts", func() {
			hijack = exec.Command(flyBin, "-t", targetedConcourse, "hijack", "-j", pipelineName+"/retry-job", "-s", "curl-server-until-3-curls", "--", "sh", "-c", "exit")
			hijackS = helpers.StartFly(hijack)
			Eventually(hijackS).Should(gbytes.Say("1: build #1, step: curl-server-until-3-curls, type: task, attempt: [1-3]"))
			Eventually(hijackS).Should(gbytes.Say("2: build #1, step: curl-server-until-3-curls, type: task, attempt: [1-3]"))
			Eventually(hijackS).Should(gbytes.Say("3: build #1, step: curl-server-until-3-curls, type: task, attempt: [1-3]"))
			hijackS.Out.Write([]byte("2"))
			Eventually(hijackS).Should(gexec.Exit())
		})
	})
})
