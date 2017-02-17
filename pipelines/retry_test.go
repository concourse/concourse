package pipelines_test

import (
	"os/exec"

	"github.com/concourse/testflight/guidserver"
	"github.com/concourse/testflight/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a step that retries", func() {
	var guidServer *guidserver.Server

	BeforeEach(func() {
		guidServer = guidserver.Start(client)

		configurePipeline(
			"-c", "fixtures/retry.yml",
			"-v", "guid-server-register-command="+guidServer.RegisterCommand(),
			"-v", "guid-server-registrations-command="+guidServer.RegistrationsCommand(),
		)
	})

	AfterEach(func() {
		guidServer.Stop()
	})

	It("retries until the step succeeds", func() {
		watch := triggerJob("retry-job")
		<-watch.Exited
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
			watch := triggerJob("retry-job-fail-for-hijacking")
			// wait until job finishes before trying to hijack
			<-watch.Exited
			Expect(watch).To(gexec.Exit(1))
		})

		It("permits hijacking a specific attempt", func() {
			hijack = exec.Command(flyBin, "-t", targetedConcourse, "hijack",
				"-j", pipelineName+"/retry-job",
				"-s", "register-server-until-3-registrations",
				"--attempt", "2",
				"--", "sh", "-c",
				"if [ `cat /tmp/retry_number` -eq 2 ]; then exit 0; else exit 1; fi;")
			hijackS = helpers.StartFly(hijack)
			Eventually(hijackS).Should(gexec.Exit(0))
		})

		It("correctly displays information about attempts", func() {
			hijack = exec.Command(flyBin, "-t", targetedConcourse, "hijack", "-j", pipelineName+"/retry-job", "-s", "register-server-until-3-registrations", "--", "sh", "-c", "exit")
			hijackS = helpers.StartFly(hijack)
			Eventually(hijackS).Should(gbytes.Say("[1-9]*: build #1, step: register-server-until-3-registrations, type: task, attempt: [1-3]"))
			Eventually(hijackS).Should(gbytes.Say("[1-9]*: build #1, step: register-server-until-3-registrations, type: task, attempt: [1-3]"))
			Eventually(hijackS).Should(gbytes.Say("[1-9]*: build #1, step: register-server-until-3-registrations, type: task, attempt: [1-3]"))
			hijackS.Out.Write([]byte("2"))
			Eventually(hijackS).Should(gexec.Exit())
		})
	})
})
