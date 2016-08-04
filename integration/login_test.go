package integration_test

import (
	"fmt"
	"io"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/atc"
	"github.com/concourse/fly/version"
)

var _ = Describe("login Command", func() {
	var (
		atcServer *ghttp.Server
	)

	Describe("login with no target name", func() {
		var (
			flyCmd *exec.Cmd
		)

		BeforeEach(func() {
			atcServer = ghttp.NewServer()
			atcServer.AppendHandlers(
				infoHandler(),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/main/auth/methods"),
					ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
				),
			)
			flyCmd = exec.Command(flyPath, "login", "-c", atcServer.URL())
		})

		It("instructs the user to specify --target", func() {
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))

			Expect(sess.Err).To(gbytes.Say(`name for the target must be specified \(--target/-t\)`))
		})
	})

	Context("with no team name", func() {
		BeforeEach(func() {
			atcServer = ghttp.NewServer()
		})

		It("falls back to atc.DefaultTeamName team", func() {
			atcServer.AppendHandlers(
				infoHandler(),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/main/auth/methods"),
					ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
				),
				tokenHandler("main"),
			)

			flyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-c", atcServer.URL())

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))
		})
	})

	Context("with a team name", func() {
		BeforeEach(func() {
			atcServer = ghttp.NewServer()
		})

		It("uses specified team", func() {
			atcServer.AppendHandlers(
				infoHandler(),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/some-team/auth/methods"),
					ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
				),
				tokenHandler("some-team"),
			)

			flyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-c", atcServer.URL(), "-n", "some-team")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))
		})

		Context("when already logged in as different team", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					infoHandler(),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/some-team/auth/methods"),
						ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
					),
					tokenHandler("some-team"),
				)

				setupFlyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-c", atcServer.URL(), "-n", "some-team")
				err := setupFlyCmd.Run()
				Expect(err).NotTo(HaveOccurred())
			})

			It("passes provided team name", func() {
				atcServer.AppendHandlers(
					infoHandler(),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/some-other-team/auth/methods"),
						ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
					),
					tokenHandler("some-other-team"),
				)

				flyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-n", "some-other-team")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			})
		})
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

		Context("when fly and atc differ in major versions", func() {
			var flyVersion string

			BeforeEach(func() {
				major, minor, patch, err := version.GetSemver(atcVersion)
				Expect(err).NotTo(HaveOccurred())

				flyVersion = fmt.Sprintf("%d.%d.%d", major+1, minor, patch)
				flyPath, err := gexec.Build(
					"github.com/concourse/fly",
					"-ldflags", fmt.Sprintf("-X github.com/concourse/fly/version.Version=%s", flyVersion),
				)
				Expect(err).NotTo(HaveOccurred())
				flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-c", atcServer.URL())
				stdin, err = flyCmd.StdinPipe()
				Expect(err).NotTo(HaveOccurred())

				atcServer.AppendHandlers(
					infoHandler(),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/auth/methods"),
						ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
					),
					tokenHandler("main"),
				)
			})

			It("warns user and does not fail", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Err).To(gbytes.Say(`fly version \(%s\) is out of sync with the target \(%s\). to sync up, run the following:`, flyVersion, atcVersion))
				Expect(sess.Err).To(gbytes.Say(`    fly -t some-target sync\n`))
			})
		})

		Context("when auth methods are returned from the API", func() {
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

					Eventually(sess.Out).Should(gbytes.Say("target saved"))

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

						Eventually(sess.Out).Should(gbytes.Say("target saved"))

						err = stdin.Close()
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					})

					Describe("running other commands", func() {
						BeforeEach(func() {
							atcServer.AppendHandlers(
								infoHandler(),
								ghttp.CombineHandlers(
									ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines"),
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
							ghttp.VerifyRequest("GET", "/api/v1/teams/main/auth/token"),
							ghttp.VerifyBasicAuth("some_username", "some_password"),
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

					_, err = fmt.Fprintf(stdin, "some_username\n")
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("password: "))

					_, err = fmt.Fprintf(stdin, "some_password\n")
					Expect(err).NotTo(HaveOccurred())

					Consistently(sess.Out.Contents).ShouldNot(ContainSubstring("some_password"))

					Eventually(sess.Out).Should(gbytes.Say("target saved"))

					err = stdin.Close()
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))
				})

				It("takes username and password as cli arguments", func() {
					flyCmd = exec.Command(flyPath,
						"-t", "some-target",
						"login", "-c", atcServer.URL(),
						"-u", "some_username",
						"-p", "some_password",
					)
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).ShouldNot(gbytes.Say("1. Basic"))
					Eventually(sess.Out).ShouldNot(gbytes.Say("2. OAuth Type 1"))
					Eventually(sess.Out).ShouldNot(gbytes.Say("3. OAuth Type 2"))
					Eventually(sess.Out).ShouldNot(gbytes.Say("choose an auth method: "))

					Eventually(sess.Out).ShouldNot(gbytes.Say("username: "))
					Eventually(sess.Out).ShouldNot(gbytes.Say("password: "))

					Eventually(sess.Out).Should(gbytes.Say("target saved"))

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

						_, err = fmt.Fprintf(stdin, "some_username\n")
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Out).Should(gbytes.Say("password: "))

						_, err = fmt.Fprintf(stdin, "some_password\n")
						Expect(err).NotTo(HaveOccurred())

						Consistently(sess.Out.Contents).ShouldNot(ContainSubstring("some_password"))

						Eventually(sess.Out).Should(gbytes.Say("target saved"))

						err = stdin.Close()
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					})

					Describe("running other commands", func() {
						BeforeEach(func() {
							atcServer.AppendHandlers(
								infoHandler(),
								ghttp.CombineHandlers(
									ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines"),
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
									ghttp.VerifyBasicAuth("some_username", "some_password"),
									ghttp.RespondWithJSONEncoded(200, atc.AuthToken{
										Type:  "Bearer",
										Value: "some-new-token",
									}),
								),
								infoHandler(),
								ghttp.CombineHandlers(
									ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines"),
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

							_, err = fmt.Fprintf(secondFlyStdin, "some_username\n")
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess.Out).Should(gbytes.Say("password: "))

							_, err = fmt.Fprintf(secondFlyStdin, "some_password\n")
							Expect(err).NotTo(HaveOccurred())

							Consistently(sess.Out.Contents).ShouldNot(ContainSubstring("some_password"))

							Eventually(sess.Out).Should(gbytes.Say("target saved"))

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

		Context("when only non-basic auth methods are returned from the API", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					infoHandler(),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/auth/methods"),
						ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{
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
			It("errors when username and password are given", func() {
				flyCmd = exec.Command(flyPath,
					"-t", "some-target",
					"login", "-c", atcServer.URL(),
					"-u", "some_username",
					"-p", "some_password",
				)
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err).Should(gbytes.Say("basic auth is not available"))

				err = stdin.Close()
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).NotTo(Equal(0))
			})
		})

		Context("when only one auth method is returned from the API", func() {
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

				Eventually(sess.Out).Should(gbytes.Say("target saved"))

				err = stdin.Close()
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			})
		})

		Context("when no auth methods are returned from the API", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					infoHandler(),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/auth/methods"),
						ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
					),
					tokenHandler("main"),
				)
			})

			It("prints a message and exits", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("target saved"))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			})

			Describe("running other commands", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						infoHandler(),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines"),
							ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
								{Name: "pipeline-1"},
							}),
						),
					)
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("target saved"))
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
					infoHandler(),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/auth/methods"),
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
