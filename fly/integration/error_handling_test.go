package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"

	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/ginkgo/v2"
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
		nonExistentTeam := "doesnotexist"
		otherTeam := "other-team"
		DescribeTable("and the team does not exist",
			func(flyCmd *exec.Cmd) {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", fmt.Sprintf("/api/v1/teams/%s", nonExistentTeam)),
						ghttp.RespondWith(http.StatusNotFound, nil),
					),
				)
				flyCmd.Path = flyPath // idk why but the .Path is not getting set when we run exec.Command even though flyPath is available...
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err.Contents).Should(ContainSubstring(`error: team 'doesnotexist' does not exist`))
			},
			Entry("checklist command returns an error",
				exec.Command(flyPath, "-t", targetName, "checklist", "-p", "pipeline", "--team", nonExistentTeam)),
			Entry("trigger-job command returns an error",
				exec.Command(flyPath, "-t", targetName, "trigger-job", "-j", "pipeline/job", "--team", nonExistentTeam)),
			Entry("expose-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "expose-pipeline", "-p", "pipeline", "--team", nonExistentTeam)),
			Entry("hide-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "hide-pipeline", "-p", "pipeline", "--team", nonExistentTeam)),
			Entry("hijack command returns an error",
				exec.Command(flyPath, "-t", targetName, "hijack", "--handle", "container-id", "--team", nonExistentTeam)),
			Entry("jobs command returns an error",
				exec.Command(flyPath, "-t", targetName, "jobs", "-p", "pipeline", "--team", nonExistentTeam)),
			Entry("pause-job command returns an error",
				exec.Command(flyPath, "-t", targetName, "pause-job", "-j", "pipeline/job", "--team", nonExistentTeam)),
			Entry("pause-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "pause-pipeline", "-p", "pipeline", "--team", nonExistentTeam)),
			Entry("unpause-job command returns an error",
				exec.Command(flyPath, "-t", targetName, "unpause-job", "-j", "pipeline/job", "--team", nonExistentTeam)),
			Entry("unpause-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "unpause-pipeline", "-p", "pipeline", "--team", nonExistentTeam)),
			Entry("set-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "set-pipeline", "-p", "pipeline", "-c", "fixtures/testConfig.yml", "--team", nonExistentTeam)),
			Entry("destroy-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "destroy-pipeline", "-p", "pipeline", "--team", nonExistentTeam)),
			Entry("get-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "get-pipeline", "-p", "pipeline", "--team", nonExistentTeam)),
			Entry("order-pipelines command returns an error",
				exec.Command(flyPath, "-t", targetName, "order-pipelines", "-p", "pipeline", "--team", nonExistentTeam)),
			Entry("abort-build command returns an error",
				exec.Command(flyPath, "-t", targetName, "abort-build", "-j", "pipeline/job", "-b", "4", "--team", nonExistentTeam)),
			Entry("archive-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "archive-pipeline", "-p", "pipeline", "--team", nonExistentTeam)),
			Entry("resources command returns an error",
				exec.Command(flyPath, "-t", targetName, "resources", "-p", "pipeline", "--team", nonExistentTeam)),
			Entry("check-resource-type command returns an error",
				exec.Command(flyPath, "-t", targetName, "check-resource-type", "-r", "mypipeline/branch:master/myresource", "--shallow", "-a", "--team", nonExistentTeam)),
			Entry("check-resource command returns an error",
				exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "--shallow", "-a", "--team", nonExistentTeam)),
			Entry("resource-versions command returns an error",
				exec.Command(flyPath, "-t", targetName, "resource-versions", "-r", "pipeline/branch:master/foo", "--team", nonExistentTeam)),
			Entry("watch command returns an error",
				exec.Command(flyPath, "-t", targetName, "watch", "-j", "pipeline/job", "--team", nonExistentTeam)),
		)

		DescribeTable("and you are NOT authorized to view the team",
			func(flyCmd *exec.Cmd) {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", fmt.Sprintf("/api/v1/teams/%s", otherTeam)),
						ghttp.RespondWith(http.StatusForbidden, nil),
					),
				)
				flyCmd.Path = flyPath // idk why but the .Path is not getting set when we run exec.Command even though flyPath is available...
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err.Contents).Should(ContainSubstring(`error: you do not have a role on team 'other-team'`))
			},
			Entry("checklist command returns an error",
				exec.Command(flyPath, "-t", targetName, "checklist", "-p", "pipeline", "--team", otherTeam)),
			Entry("trigger-job command returns an error",
				exec.Command(flyPath, "-t", targetName, "trigger-job", "-j", "pipeline/job", "--team", otherTeam)),
			Entry("expose-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "expose-pipeline", "-p", "pipeline", "--team", otherTeam)),
			Entry("hide-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "hide-pipeline", "-p", "pipeline", "--team", otherTeam)),
			Entry("hijack command returns an error",
				exec.Command(flyPath, "-t", targetName, "hijack", "--handle", "container-id", "--team", otherTeam)),
			Entry("jobs command returns an error",
				exec.Command(flyPath, "-t", targetName, "jobs", "-p", "pipeline", "--team", otherTeam)),
			Entry("pause-job command returns an error",
				exec.Command(flyPath, "-t", targetName, "pause-job", "-j", "pipeline/job", "--team", otherTeam)),
			Entry("pause-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "pause-pipeline", "-p", "pipeline", "--team", otherTeam)),
			Entry("unpause-job command returns an error",
				exec.Command(flyPath, "-t", targetName, "unpause-job", "-j", "pipeline/job", "--team", otherTeam)),
			Entry("unpause-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "unpause-pipeline", "-p", "pipeline", "--team", otherTeam)),
			Entry("set-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "set-pipeline", "-p", "pipeline", "-c", "fixtures/testConfig.yml", "--team", otherTeam)),
			Entry("destroy-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "destroy-pipeline", "-p", "pipeline", "--team", otherTeam)),
			Entry("get-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "get-pipeline", "-p", "pipeline", "--team", otherTeam)),
			Entry("abort-build command returns an error",
				exec.Command(flyPath, "-t", targetName, "abort-build", "-j", "pipeline/job", "-b", "4", "--team", otherTeam)),
			Entry("archive-pipeline command returns an error",
				exec.Command(flyPath, "-t", targetName, "archive-pipeline", "-p", "pipeline", "--team", otherTeam)),
			Entry("resources command returns an error",
				exec.Command(flyPath, "-t", targetName, "resources", "-p", "pipeline", "--team", otherTeam)),
			Entry("check-resource-type command returns an error",
				exec.Command(flyPath, "-t", targetName, "check-resource-type", "-r", "mypipeline/branch:master/myresource", "--shallow", "-a", "--team", otherTeam)),
			Entry("check-resource command returns an error",
				exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "--shallow", "-a", "--team", otherTeam)),
			Entry("resource-versions command returns an error",
				exec.Command(flyPath, "-t", targetName, "resource-versions", "-r", "pipeline/branch:master/foo", "--team", otherTeam)),
			Entry("watch command returns an error",
				exec.Command(flyPath, "-t", targetName, "watch", "-j", "pipeline/job", "--team", otherTeam)),
		)
	})
})
