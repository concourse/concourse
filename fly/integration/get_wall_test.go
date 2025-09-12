package integration_test

import (
	"os/exec"
	"time"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("FLY CLI", func() {
	Describe("get-wall", func() {
		Context("when a wall message is set", func() {
			It("shows the message and ttl", func() {
				wall := atc.Wall{Message: "test message", TTL: time.Hour}
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/wall"),
						ghttp.RespondWithJSONEncoded(200, wall),
					),
				)

				flyCmd := exec.Command(flyPath, "-t", targetName, "get-wall")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
				Expect(sess.Out).To(gbytes.Say("Wall Message: test message"))
				Expect(sess.Out).To(gbytes.Say("Expires in:"))
			})
		})

		Context("when no wall message is set", func() {
			It("prints a helpful message", func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/wall"),
						ghttp.RespondWithJSONEncoded(200, atc.Wall{}),
					),
				)
				flyCmd := exec.Command(flyPath, "-t", targetName, "get-wall")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
				Expect(sess.Out).To(gbytes.Say("No wall message is currently set"))
			})
		})

		Context("when the API returns an error", func() {
			It("exits non-zero and shows an error", func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/wall"),
						ghttp.RespondWith(500, ""),
					),
				)
				flyCmd := exec.Command(flyPath, "-t", targetName, "get-wall")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
				Expect(sess.Err).To(gbytes.Say("failed to get wall message"))
			})
		})
	})
})
