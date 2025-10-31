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
	Describe("set-wall", func() {
		Context("when setting a wall message succeeds with ttl", func() {
			It("set wall and print success", func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/wall"),
						ghttp.RespondWithJSONEncoded(200, atc.Wall{Message: "test message", TTL: time.Hour}),
					),
				)

				flyCmd := exec.Command(flyPath, "-t", targetName, "set-wall", "-m", "test message", "--ttl", "1h")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
				Expect(sess.Out).To(gbytes.Say("Wall message set successfully"))
			})
		})

		Context("when setting a wall message succeeds without ttl", func() {
			It("set wall and print success", func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/wall"),
						ghttp.RespondWithJSONEncoded(200, atc.Wall{Message: "test message", TTL: time.Hour}),
					),
				)

				flyCmd := exec.Command(flyPath, "-t", targetName, "set-wall", "-m", "test message")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
				Expect(sess.Out).To(gbytes.Say("Wall message set successfully"))
			})
		})

		Context("when the API returns an error", func() {
			It("exits non-zero and shows an error", func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/wall"),
						ghttp.RespondWith(500, ""),
					),
				)
				flyCmd := exec.Command(flyPath, "-t", targetName, "set-wall", "-m", "test message", "--ttl", "1h")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
				Expect(sess.Err).To(gbytes.Say("failed to set wall message"))
			})
		})
	})
})
