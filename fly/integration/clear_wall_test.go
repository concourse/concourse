package integration_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("FLY CLI", func() {
	Describe("clear-wall", func() {
		Context("when clearing succeeds", func() {
			It("sends DELETE /api/v1/wall and prints success", func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/api/v1/wall"),
						ghttp.RespondWith(200, ""),
					),
				)

				flyCmd := exec.Command(flyPath, "-t", targetName, "clear-wall")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
				Expect(sess.Out).To(gbytes.Say("Wall message cleared successfully"))
			})
		})

		Context("when the API returns an error", func() {
			It("exits non-zero and shows an error", func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/api/v1/wall"),
						ghttp.RespondWith(500, ""),
					),
				)
				flyCmd := exec.Command(flyPath, "-t", targetName, "clear-wall")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
				Expect(sess.Err).To(gbytes.Say("failed to clear wall message"))
			})
		})
	})
})
