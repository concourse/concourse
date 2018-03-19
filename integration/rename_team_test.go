package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("RenameTeam", func() {
	var newName string
	BeforeEach(func() {
		expectedURL := "/api/v1/teams/a-team/rename"
		newName = "b-team"

		atcServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("PUT", expectedURL),
				ghttp.VerifyJSON(fmt.Sprintf(`{"name":%q}`, newName)),
				ghttp.RespondWith(http.StatusNoContent, ""),
			),
		)
	})

	Context("when not specifying a team name", func() {
		It("fails and says you should provide a team name", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "rename-team", "-n", "b-team")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))

			Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("o", "old-name") + "' was not specified"))
		})
	})

	Context("when not specifying a new name", func() {
		It("fails and says you should provide a new name for the team", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "rename-team", "-o", "a-team")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))

			Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("n", "new-name") + "' was not specified"))
		})
	})

	Context("when all the inputs are provided", func() {
		It("successfully renames the team to the provided name", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "rename-team", "-o", "a-team", "-n", newName)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(0))
			Expect(atcServer.ReceivedRequests()).To(HaveLen(4))
			Expect(sess.Out).To(gbytes.Say(fmt.Sprintf("Team successfully renamed to %s", newName)))
		})

		Context("when the team is not found", func() {
			BeforeEach(func() {
				atcServer.SetHandler(3, ghttp.RespondWith(http.StatusNotFound, ""))
			})

			It("returns an error", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "rename-team", "-o", "a-team", "-n", newName)

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))
				Expect(atcServer.ReceivedRequests()).To(HaveLen(4))
				Expect(sess.Err).To(gbytes.Say("Team 'a-team' not found"))
			})
		})

		Context("when an error occurs", func() {
			BeforeEach(func() {
				atcServer.SetHandler(3, ghttp.RespondWith(http.StatusTeapot, ""))
			})

			It("returns an error", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "rename-team", "-o", "a-team", "-n", newName)

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))
				Expect(atcServer.ReceivedRequests()).To(HaveLen(4))
				Expect(sess.Err).To(gbytes.Say("418 I'm a teapot"))
			})
		})
	})
})
