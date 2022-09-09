package integration_test

import (
	"net/http"
	"os/exec"

	"github.com/concourse/concourse/fly/rc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("status Command", func() {
	var (
		flyCmd *exec.Cmd
	)

	BeforeEach(func() {
		createFlyRc(rc.Targets{
			"with-token": {
				API:      atcServer.URL(),
				TeamName: "test",
				Token:    &rc.TargetToken{Type: "Bearer", Value: validAccessToken(date(2020, 1, 1))},
			},
			"without-token": {
				API:      "https://example.com/another-test",
				TeamName: "test",
				Token:    &rc.TargetToken{},
			},
		})
	})

	Context("status with no target name", func() {
		var (
			flyCmd *exec.Cmd
		)
		BeforeEach(func() {
			flyCmd = exec.Command(flyPath, "status")
		})

		It("instructs the user to specify --target", func() {
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))

			Expect(sess.Err).To(gbytes.Say(`no target specified. specify the target with -t`))
		})
	})

	Context("status with target name", func() {
		Context("when target is saved with valid token", func() {
			BeforeEach(func() {
				atcServer.Reset()
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/user"),
						ghttp.RespondWithJSONEncoded(200, map[string]interface{}{"team": "test"}),
					),
				)
			})

			It("the command succeeds", func() {
				flyCmd = exec.Command(flyPath, "-t", "with-token", "status")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				Expect(sess.Out).To(gbytes.Say(`logged in successfully`))
			})
		})

		Context("when target is saved with a token that is rejected by the server", func() {
			BeforeEach(func() {
				atcServer.Reset()
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/user"),
						ghttp.RespondWith(http.StatusUnauthorized, nil),
					),
				)
			})

			It("the command fails", func() {
				flyCmd = exec.Command(flyPath, "-t", "with-token", "status")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say(`please login again`))
				Expect(sess.Err).To(gbytes.Say(`token validation failed with error: not authorized`))
			})
		})

		Context("when target is saved with invalid token", func() {
			It("the command fails", func() {
				flyCmd = exec.Command(flyPath, "-t", "without-token", "status")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say(`logged out`))
			})
		})
	})
})
