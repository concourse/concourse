package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/atc"
	"github.com/concourse/fly/pty"
)

var _ = Describe("login Command", func() {
	var (
		atcServer *ghttp.Server

		homeDir string
	)

	BeforeEach(func() {
		var err error

		homeDir, err = ioutil.TempDir("", "fly-test")
		Expect(err).NotTo(HaveOccurred())

		os.Setenv("HOME", homeDir)
		os.Setenv("HOMEPATH", homeDir)
	})

	AfterEach(func() {
		os.RemoveAll(homeDir)

		os.Unsetenv("HOME")
		os.Unsetenv("HOMEPATH")
	})

	Describe("login", func() {
		var (
			flyCmd *exec.Cmd
			flyPTY pty.PTY
		)

		BeforeEach(func() {
			atcServer = ghttp.NewServer()
			flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-c", atcServer.URL())

			p, err := pty.Open()
			Expect(err).NotTo(HaveOccurred())

			flyCmd.Stdin = p.TTYR
			flyPTY = p
		})

		Context("when auth methods are returned from the API", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/auth/methods"),
						ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{
							{
								Type:        atc.AuthTypeBasic,
								DisplayName: "Basic",
								AuthURL:     "https://example.com/login/basic",
							},
							{
								Type:        atc.AuthTypeOAuth,
								DisplayName: "OAuth Type 1",
								AuthURL:     "https://example.com/auth/oauth-1",
							},
							{
								Type:        atc.AuthTypeOAuth,
								DisplayName: "OAuth Type 2",
								AuthURL:     "https://example.com/auth/oauth-2",
							},
						}),
					),
				)
			})

			Context("when an OAuth method is chosen", func() {
				It("asks for manual token entry for oauth methods", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("1. Basic"))
					Eventually(sess.Out).Should(gbytes.Say("2. OAuth Type 1"))
					Eventually(sess.Out).Should(gbytes.Say("3. OAuth Type 2"))
					Eventually(sess.Out).Should(gbytes.Say("choose an auth method: "))

					_, err = fmt.Fprintf(flyPTY.PTYW, "3\r")
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("navigate to the following URL in your browser:"))
					Eventually(sess.Out).Should(gbytes.Say("    https://example.com/auth/oauth-2"))
					Eventually(sess.Out).Should(gbytes.Say("enter token: "))

					_, err = fmt.Fprintf(flyPTY.PTYW, "some token that i totally got via legitimate means\r")
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("token saved"))

					err = flyPTY.PTYW.Close()
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))
				})
			})

			Context("when a Basic method is chosen", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/auth/token"),
							ghttp.VerifyBasicAuth("some username", "some password"),
							ghttp.RespondWithJSONEncoded(200, atc.AuthToken{
								Type:  "Bearer",
								Value: "some-token",
							}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/pipelines"),
							ghttp.VerifyHeaderKV("Authorization", "Bearer some-token"),
							ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
								{Name: "pipeline-1"},
							}),
						),
					)
				})

				It("asks for username and password for basic methods", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("1. Basic"))
					Eventually(sess.Out).Should(gbytes.Say("2. OAuth Type 1"))
					Eventually(sess.Out).Should(gbytes.Say("3. OAuth Type 2"))
					Eventually(sess.Out).Should(gbytes.Say("choose an auth method: "))

					_, err = fmt.Fprintf(flyPTY.PTYW, "1\r")
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("username: "))

					_, err = fmt.Fprintf(flyPTY.PTYW, "some username\r")
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("password: "))

					_, err = fmt.Fprintf(flyPTY.PTYW, "some password\r")
					Expect(err).NotTo(HaveOccurred())

					Consistently(sess.Out.Contents).ShouldNot(ContainSubstring("some password"))

					Eventually(sess.Out).Should(gbytes.Say("token saved"))

					err = flyPTY.PTYW.Close()
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))

					otherCmd := exec.Command(flyPath, "-t", "some-target", "pipelines")

					sess, err = gexec.Start(otherCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited

					Expect(sess).To(gbytes.Say("pipeline-1"))

					Expect(sess.ExitCode()).To(Equal(0))
				})
			})
		})

		Context("and the api returns an internal server error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/auth/methods"),
						ghttp.RespondWith(500, ""),
					),
				)
			})

			It("writes an error message to stderr", func() {
				sess, err := gexec.Start(flyCmd, nil, nil)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess.Err).Should(gbytes.Say("Unexpected Response"))
				Eventually(sess).Should(gexec.Exit(1))
			})
		})
	})
})
