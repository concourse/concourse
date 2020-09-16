package integration_test

import (
	"net/http"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	Describe("auth failures", func() {
		var (
			flyCmd *exec.Cmd
		)

		BeforeEach(func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "containers")
		})

		Context("when a 401 response is received", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/containers"),
						ghttp.RespondWith(401, ""),
					),
				)
			})

			It("instructs the user to log in", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say("not authorized\\. run the following to log in:\n\n    "))
				Expect(sess.Err).To(gbytes.Say(`fly.* -t ` + targetName + ` login`))
			})
		})
	})

	Describe("missing target", func() {
		var (
			flyCmd *exec.Cmd
		)

		BeforeEach(func() {
			flyCmd = exec.Command(flyPath, "containers")
		})

		It("instructs the user to specify a target", func() {
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))

			Expect(sess.Err).To(gbytes.Say("no target specified\\. specify the target with -t or log in like so:"))
			Expect(sess.Err).To(gbytes.Say(`fly.* -t \(alias\) login -c \(concourse url\)`))
		})
	})

	Describe("network errors", func() {
		var (
			flyCmd *exec.Cmd
		)

		BeforeEach(func() {
			atcServer.Close()

			flyCmd = exec.Command(flyPath, "-t", targetName, "containers")
		})

		It("tells the user a network error occurred, and that their target may be wrong, and makes fun of them", func() {
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))

			Expect(sess.Err).To(gbytes.Say("could not reach the Concourse server called " + targetName))
			Expect(sess.Err).To(gbytes.Say("lol"))
		})
	})

	Context("when --team is set", func() {
		DescribeTable("and the team does not exist",
			func(flyCmd *exec.Cmd) {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/doesnotexist"),
						ghttp.RespondWith(http.StatusNotFound, nil),
					),
				)
				flyCmd.Path = flyPath // idk why but the .Path is not getting set when we run exec.Command even though flyPath is available...
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err.Contents).Should(ContainSubstring(`error: team 'doesnotexist' does not exist`))
			},
			Entry("trigger-job command returns an error",
				exec.Command(flyPath, "-t", targetName, "trigger-job", "-j", "pipeline/job", "--team", "doesnotexist")),
			Entry("hide-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "hide-pipeline", "-p", "pipeline", "--team", "doesnotexist")),
			Entry("hijack command returns an error",
				exec.Command(flyPath, "-t", targetName, "hijack", "--handle", "container-id", "--team", "doesnotexist")),
			Entry("jobs command returns an error",
				exec.Command(flyPath, "-t", targetName, "jobs", "-p", "pipeline", "--team", "doesnotexist")),
			Entry("pause-job command returns an error",
				exec.Command(flyPath, "-t", targetName, "pause-job", "-j", "pipeline/job", "--team", "doesnotexist")),
			Entry("pause-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "pause-pipeline", "-p", "pipeline", "--team", "doesnotexist")),
			Entry("unpause-job command returns an error",
				exec.Command(flyPath, "-t", targetName, "unpause-job", "-j", "pipeline/job", "--team", "doesnotexist")),
			Entry("unpause-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "unpause-pipeline", "-p", "pipeline", "--team", "doesnotexist")),
			Entry("set-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "set-pipeline", "-p", "pipeline", "-c", "fixtures/testConfig.yml", "--team", "doesnotexist")),
			Entry("destroy-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "destroy-pipeline", "-p", "pipeline", "--team", "doesnotexist")),
		)

		DescribeTable("and you are NOT authorized to view the team",
			func(flyCmd *exec.Cmd) {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/other-team"),
						ghttp.RespondWith(http.StatusForbidden, nil),
					),
				)
				flyCmd.Path = flyPath // idk why but the .Path is not getting set when we run exec.Command even though flyPath is available...
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err.Contents).Should(ContainSubstring(`error: you do not have a role on team 'other-team'`))
			},
			Entry("trigger-job command returns an error",
				exec.Command(flyPath, "-t", targetName, "trigger-job", "-j", "pipeline/job", "--team", "other-team")),
			Entry("hide-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "hide-pipeline", "-p", "pipeline", "--team", "other-team")),
			Entry("hijack command returns an error",
				exec.Command(flyPath, "-t", targetName, "hijack", "--handle", "container-id", "--team", "other-team")),
			Entry("jobs command returns an error",
				exec.Command(flyPath, "-t", targetName, "jobs", "-p", "pipeline", "--team", "other-team")),
			Entry("pause-job command returns an error",
				exec.Command(flyPath, "-t", targetName, "pause-job", "-j", "pipeline/job", "--team", "other-team")),
			Entry("pause-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "pause-pipeline", "-p", "pipeline", "--team", "other-team")),
			Entry("unpause-job command returns an error",
				exec.Command(flyPath, "-t", targetName, "unpause-job", "-j", "pipeline/job", "--team", "other-team")),
			Entry("unpause-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "unpause-pipeline", "-p", "pipeline", "--team", "other-team")),
			Entry("set-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "set-pipeline", "-p", "pipeline", "-c", "fixtures/testConfig.yml", "--team", "other-team")),
			Entry("destroy-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "destroy-pipeline", "-p", "pipeline", "--team", "other-team")),
		)
	})
})
