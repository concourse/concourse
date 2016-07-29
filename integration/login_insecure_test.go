package integration_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/atc"
)

var _ = Describe("login -k Command", func() {
	var atcServer *ghttp.Server

	Describe("login", func() {
		var (
			flyCmd *exec.Cmd
			stdin  io.WriteCloser
		)
		BeforeEach(func() {
			l := log.New(GinkgoWriter, "TLSServer", 0)
			atcServer = ghttp.NewUnstartedServer()
			atcServer.HTTPTestServer.Config.ErrorLog = l
			atcServer.HTTPTestServer.StartTLS()
		})

		AfterEach(func() {
			atcServer.Close()
		})

		Context("to new target with invalid SSL with -k", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					infoHandler(),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/auth/methods"),
						ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{
							{
								Type:        atc.AuthTypeBasic,
								DisplayName: "Basic",
								AuthURL:     "https://example.com/login/basic",
							},
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/auth/token"),
						ghttp.VerifyBasicAuth("some username", "some password"),
						ghttp.RespondWithJSONEncoded(200, atc.AuthToken{
							Type:  "Bearer",
							Value: "some-token",
						}),
					),
				)

				flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-c", atcServer.URL(), "-k")

				var err error
				stdin, err = flyCmd.StdinPipe()
				Expect(err).NotTo(HaveOccurred())
			})

			It("succeeds", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("username: "))

				_, err = fmt.Fprintf(stdin, "some username\n")
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("password: "))

				_, err = fmt.Fprintf(stdin, "some password\n")
				Expect(err).NotTo(HaveOccurred())

				Consistently(sess.Out.Contents).ShouldNot(ContainSubstring("some password"))

				Eventually(sess.Out).Should(gbytes.Say("target saved"))

				err = stdin.Close()
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			})

			Context("login to existing target", func() {
				var otherCmd *exec.Cmd
				BeforeEach(func() {
					atcServer.AppendHandlers(
						infoHandler(),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/main/auth/methods"),
							ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{
								{
									Type:        atc.AuthTypeBasic,
									DisplayName: "Basic",
									AuthURL:     "https://example.com/login/basic",
								},
							}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/main/auth/token"),
							ghttp.VerifyBasicAuth("some username", "some password"),
							ghttp.RespondWithJSONEncoded(200, atc.AuthToken{
								Type:  "Bearer",
								Value: "some-token",
							}),
						),
					)

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess.Out).Should(gbytes.Say("username: "))
					_, err = fmt.Fprintf(stdin, "some username\n")
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess.Out).Should(gbytes.Say("password: "))
					_, err = fmt.Fprintf(stdin, "some password\n")
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess.Out).Should(gbytes.Say("target saved"))

					err = stdin.Close()
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))
				})

				Context("with -k", func() {
					BeforeEach(func() {
						otherCmd = exec.Command(flyPath, "-t", "some-target", "login", "-k")

						var err error
						stdin, err = otherCmd.StdinPipe()
						Expect(err).NotTo(HaveOccurred())
					})

					It("succeeds", func() {
						sess, err := gexec.Start(otherCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Out).Should(gbytes.Say("username: "))

						_, err = fmt.Fprintf(stdin, "some username\n")
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Out).Should(gbytes.Say("password: "))

						_, err = fmt.Fprintf(stdin, "some password\n")
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Out).Should(gbytes.Say("target saved"))

						err = stdin.Close()
						Expect(err).NotTo(HaveOccurred())

						err = stdin.Close()
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					})
				})

				Context("without -k", func() {
					BeforeEach(func() {
						otherCmd = exec.Command(flyPath, "-t", "some-target", "login")
					})

					It("errors", func() {
						sess, err := gexec.Start(otherCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(1))
						Eventually(sess.Err).Should(gbytes.Say("x509: certificate signed by unknown authority"))
					})
				})
			})

		})

		Context("to new target with invalid SSL without -k", func() {
			BeforeEach(func() {
				flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-c", atcServer.URL())

				var err error
				stdin, err = flyCmd.StdinPipe()
				Expect(err).NotTo(HaveOccurred())
			})

			It("errors", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				err = stdin.Close()
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
				Eventually(sess.Err).Should(gbytes.Say("x509: certificate signed by unknown authority"))
			})
		})

		Context("to existing target with invalid SSL certificate", func() {
			Context("when 'insecure' is not set", func() {
				BeforeEach(func() {
					flyrcContents := `targets:
  some-target:
    api: ` + atcServer.URL() + `
    team: main
    token:
      type: Bearer
      value: some-token`
					ioutil.WriteFile(homeDir+"/.flyrc", []byte(flyrcContents), 0777)
				})

				Context("with -k", func() {
					BeforeEach(func() {
						atcServer.AppendHandlers(
							infoHandler(),
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("GET", "/api/v1/teams/main/auth/methods"),
								ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{
									{
										Type:        atc.AuthTypeBasic,
										DisplayName: "Basic",
										AuthURL:     "https://example.com/login/basic",
									},
								}),
							),
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("GET", "/api/v1/teams/main/auth/token"),
								ghttp.VerifyBasicAuth("some username", "some password"),
								ghttp.RespondWithJSONEncoded(200, atc.AuthToken{
									Type:  "Bearer",
									Value: "some-token",
								}),
							),
						)

						flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-k")

						var err error
						stdin, err = flyCmd.StdinPipe()
						Expect(err).NotTo(HaveOccurred())
					})

					It("succeeds", func() {
						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Out).Should(gbytes.Say("username: "))

						_, err = fmt.Fprintf(stdin, "some username\n")
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Out).Should(gbytes.Say("password: "))

						_, err = fmt.Fprintf(stdin, "some password\n")
						Expect(err).NotTo(HaveOccurred())

						Consistently(sess.Out.Contents).ShouldNot(ContainSubstring("some password"))

						Eventually(sess.Out).Should(gbytes.Say("target saved"))

						err = stdin.Close()
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					})
				})

				Context("without -k", func() {
					BeforeEach(func() {
						flyCmd = exec.Command(flyPath, "-t", "some-target", "login")
					})

					It("errors", func() {
						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(1))
						Eventually(sess.Err).Should(gbytes.Say("x509: certificate signed by unknown authority"))
					})
				})
			})
		})
	})
})
