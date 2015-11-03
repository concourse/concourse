package integration_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/atc"
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

		if runtime.GOOS == "windows" {
			os.Setenv("USERPROFILE", homeDir)
		} else {
			os.Setenv("HOME", homeDir)
		}
	})

	AfterEach(func() {
		os.RemoveAll(homeDir)
	})

	Describe("login", func() {
		var (
			flyCmd *exec.Cmd
			stdin  io.WriteCloser
		)

		BeforeEach(func() {
			atcServer = ghttp.NewServer()
			flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-c", atcServer.URL())

			var err error
			stdin, err = flyCmd.StdinPipe()
			Expect(err).NotTo(HaveOccurred())
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

					_, err = fmt.Fprintf(stdin, "3\n")
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("navigate to the following URL in your browser:"))
					Eventually(sess.Out).Should(gbytes.Say("    https://example.com/auth/oauth-2"))
					Eventually(sess.Out).Should(gbytes.Say("enter token: "))

					_, err = fmt.Fprintf(stdin, "bogustoken\n")
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("token must be of the format 'TYPE VALUE', e.g. 'Bearer ...'"))

					_, err = fmt.Fprintf(stdin, "Bearer grylls\n")
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("token saved"))

					err = stdin.Close()
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))
				})

				Context("after logging in succeeds", func() {
					BeforeEach(func() {
						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Out).Should(gbytes.Say("1. Basic"))
						Eventually(sess.Out).Should(gbytes.Say("2. OAuth Type 1"))
						Eventually(sess.Out).Should(gbytes.Say("3. OAuth Type 2"))
						Eventually(sess.Out).Should(gbytes.Say("choose an auth method: "))

						_, err = fmt.Fprintf(stdin, "3\n")
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Out).Should(gbytes.Say("enter token: "))

						_, err = fmt.Fprintf(stdin, "Bearer some-entered-token\n")
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Out).Should(gbytes.Say("token saved"))

						err = stdin.Close()
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					})

					Describe("running other commands", func() {
						BeforeEach(func() {
							atcServer.AppendHandlers(
								ghttp.CombineHandlers(
									ghttp.VerifyRequest("GET", "/api/v1/pipelines"),
									ghttp.VerifyHeaderKV("Authorization", "Bearer some-entered-token"),
									ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
										{Name: "pipeline-1"},
									}),
								),
							)
						})

						It("uses the saved token", func() {
							otherCmd := exec.Command(flyPath, "-t", "some-target", "pipelines")

							sess, err := gexec.Start(otherCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							<-sess.Exited

							Expect(sess).To(gbytes.Say("pipeline-1"))

							Expect(sess.ExitCode()).To(Equal(0))
						})
					})
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
					)
				})

				It("asks for username and password for basic methods", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("1. Basic"))
					Eventually(sess.Out).Should(gbytes.Say("2. OAuth Type 1"))
					Eventually(sess.Out).Should(gbytes.Say("3. OAuth Type 2"))
					Eventually(sess.Out).Should(gbytes.Say("choose an auth method: "))

					_, err = fmt.Fprintf(stdin, "1\n")
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("username: "))

					_, err = fmt.Fprintf(stdin, "some username\n")
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("password: "))

					_, err = fmt.Fprintf(stdin, "some password\n")
					Expect(err).NotTo(HaveOccurred())

					Consistently(sess.Out.Contents).ShouldNot(ContainSubstring("some password"))

					Eventually(sess.Out).Should(gbytes.Say("token saved"))

					err = stdin.Close()
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))
				})

				Context("after logging in succeeds", func() {
					BeforeEach(func() {
						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Out).Should(gbytes.Say("1. Basic"))
						Eventually(sess.Out).Should(gbytes.Say("2. OAuth Type 1"))
						Eventually(sess.Out).Should(gbytes.Say("3. OAuth Type 2"))
						Eventually(sess.Out).Should(gbytes.Say("choose an auth method: "))

						_, err = fmt.Fprintf(stdin, "1\n")
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Out).Should(gbytes.Say("username: "))

						_, err = fmt.Fprintf(stdin, "some username\n")
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Out).Should(gbytes.Say("password: "))

						_, err = fmt.Fprintf(stdin, "some password\n")
						Expect(err).NotTo(HaveOccurred())

						Consistently(sess.Out.Contents).ShouldNot(ContainSubstring("some password"))

						Eventually(sess.Out).Should(gbytes.Say("token saved"))

						err = stdin.Close()
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					})

					Describe("running other commands", func() {
						BeforeEach(func() {
							atcServer.AppendHandlers(
								ghttp.CombineHandlers(
									ghttp.VerifyRequest("GET", "/api/v1/pipelines"),
									ghttp.VerifyHeaderKV("Authorization", "Bearer some-token"),
									ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
										{Name: "pipeline-1"},
									}),
								),
							)
						})

						It("uses the saved token", func() {
							otherCmd := exec.Command(flyPath, "-t", "some-target", "pipelines")

							sess, err := gexec.Start(otherCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							<-sess.Exited

							Expect(sess).To(gbytes.Say("pipeline-1"))

							Expect(sess.ExitCode()).To(Equal(0))
						})
					})

					Describe("logging in again with the same target", func() {
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
									}),
								),
								ghttp.CombineHandlers(
									ghttp.VerifyRequest("GET", "/api/v1/auth/token"),
									ghttp.VerifyBasicAuth("some username", "some password"),
									ghttp.RespondWithJSONEncoded(200, atc.AuthToken{
										Type:  "Bearer",
										Value: "some-new-token",
									}),
								),
								ghttp.CombineHandlers(
									ghttp.VerifyRequest("GET", "/api/v1/pipelines"),
									ghttp.VerifyHeaderKV("Authorization", "Bearer some-new-token"),
									ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
										{Name: "pipeline-1"},
									}),
								),
							)
						})

						It("updates the token", func() {
							loginAgainCmd := exec.Command(flyPath, "-t", "some-target", "login")

							secondFlyStdin, err := loginAgainCmd.StdinPipe()
							Expect(err).NotTo(HaveOccurred())

							sess, err := gexec.Start(loginAgainCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess.Out).Should(gbytes.Say("username: "))

							_, err = fmt.Fprintf(secondFlyStdin, "some username\n")
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess.Out).Should(gbytes.Say("password: "))

							_, err = fmt.Fprintf(secondFlyStdin, "some password\n")
							Expect(err).NotTo(HaveOccurred())

							Consistently(sess.Out.Contents).ShouldNot(ContainSubstring("some password"))

							Eventually(sess.Out).Should(gbytes.Say("token saved"))

							err = secondFlyStdin.Close()
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
			})
		})

		Context("when only one auth method is returned from the API", func() {
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
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/auth/token"),
						ghttp.VerifyBasicAuth("some username", "some password"),
						ghttp.RespondWithJSONEncoded(200, atc.AuthToken{
							Type:  "Bearer",
							Value: "some-token",
						}),
					),
				)
			})

			It("uses its auth method without asking", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("username: "))

				_, err = fmt.Fprintf(stdin, "some username\n")
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("password: "))

				_, err = fmt.Fprintf(stdin, "some password\n")
				Expect(err).NotTo(HaveOccurred())

				Consistently(sess.Out.Contents).ShouldNot(ContainSubstring("some password"))

				Eventually(sess.Out).Should(gbytes.Say("token saved"))

				err = stdin.Close()
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			})
		})

		Context("when no auth methods are returned from the API", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/auth/methods"),
						ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
					),
				)
			})

			It("prints a message and exits", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("no auth methods configured; updating target data"))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			})

			Describe("running other commands", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/pipelines"),
							ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
								{Name: "pipeline-1"},
							}),
						),
					)
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("no auth methods configured; updating target data"))
				})

				It("uses the saved target", func() {
					otherCmd := exec.Command(flyPath, "-t", "some-target", "pipelines")

					sess, err := gexec.Start(otherCmd, GinkgoWriter, GinkgoWriter)
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
