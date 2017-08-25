package integration_test

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"regexp"

	"github.com/concourse/atc"
	"github.com/concourse/fly/version"
)

var _ = Describe("login Command", func() {
	var (
		loginATCServer *ghttp.Server
		tmpDir         string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "fly-test")
		Expect(err).ToNot(HaveOccurred())

		os.Setenv("HOME", tmpDir)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("login with no target name", func() {
		var (
			flyCmd *exec.Cmd
		)

		BeforeEach(func() {
			loginATCServer = ghttp.NewServer()
			loginATCServer.AppendHandlers(
				infoHandler(),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/main/auth/methods"),
					ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
				),
			)
			flyCmd = exec.Command(flyPath, "login", "-c", loginATCServer.URL())
		})

		AfterEach(func() {
			loginATCServer.Close()
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
			loginATCServer = ghttp.NewServer()
		})

		AfterEach(func() {
			loginATCServer.Close()
		})

		It("falls back to atc.DefaultTeamName team", func() {
			loginATCServer.AppendHandlers(
				infoHandler(),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/main/auth/methods"),
					ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
				),
				tokenHandler("main"),
			)

			flyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL())

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gbytes.Say("logging in to team 'main'"))

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))
		})

		Context("when already logged in as different team", func() {
			BeforeEach(func() {
				loginATCServer.AppendHandlers(
					infoHandler(),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/some-team/auth/methods"),
						ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
					),
					tokenHandler("some-team"),
				)

				setupFlyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "-n", "some-team")
				err := setupFlyCmd.Run()
				Expect(err).NotTo(HaveOccurred())
			})

			It("uses the saved team name", func() {
				loginATCServer.AppendHandlers(
					infoHandler(),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/some-team/auth/methods"),
						ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
					),
					tokenHandler("some-team"),
				)

				flyCmd := exec.Command(flyPath, "-t", "some-target", "login")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gbytes.Say("logging in to team 'some-team'"))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			})

		})

	})

	Context("with a team name", func() {
		BeforeEach(func() {
			loginATCServer = ghttp.NewServer()
		})

		AfterEach(func() {
			loginATCServer.Close()
		})

		It("uses specified team", func() {
			loginATCServer.AppendHandlers(
				infoHandler(),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/some-team/auth/methods"),
					ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
				),
				tokenHandler("some-team"),
			)

			flyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "-n", "some-team")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gbytes.Say("logging in to team 'some-team'"))

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))
		})

		Context("when tracing is not enabled", func() {
			It("does not print out API calls", func() {
				loginATCServer.AppendHandlers(
					infoHandler(),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/some-team/auth/methods"),
						ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
					),
					tokenHandler("some-team"),
				)

				flyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "-n", "some-team")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Consistently(sess.Err).ShouldNot(gbytes.Say("GET /api/v1/teams/some-team/auth/methods HTTP/1.1"))
				Consistently(sess.Out).ShouldNot(gbytes.Say("GET /api/v1/teams/some-team/auth/methods HTTP/1.1"))
				Consistently(sess.Err).ShouldNot(gbytes.Say("HTTP/1.1 200 OK"))
				Consistently(sess.Out).ShouldNot(gbytes.Say("HTTP/1.1 200 OK"))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			})
		})

		Context("when tracing is enabled", func() {
			It("prints out API calls", func() {
				loginATCServer.AppendHandlers(
					infoHandler(),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/some-team/auth/methods"),
						ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
					),
					tokenHandler("some-team"),
				)

				flyCmd := exec.Command(flyPath, "--verbose", "-t", "some-target", "login", "-c", loginATCServer.URL(), "-n", "some-team")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err).Should(gbytes.Say("GET /api/v1/teams/some-team/auth/methods HTTP/1.1"))
				Eventually(sess.Err).Should(gbytes.Say("HTTP/1.1 200 OK"))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			})
		})

		Context("when already logged in as different team", func() {
			BeforeEach(func() {
				loginATCServer.AppendHandlers(
					infoHandler(),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/some-team/auth/methods"),
						ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
					),
					tokenHandler("some-team"),
				)

				setupFlyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "-n", "some-team")
				err := setupFlyCmd.Run()
				Expect(err).NotTo(HaveOccurred())
			})

			It("passes provided team name", func() {
				loginATCServer.AppendHandlers(
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

	Describe("with ca cert", func() {
		BeforeEach(func() {
			loginATCServer = ghttp.NewUnstartedServer()
			cert, err := tls.X509KeyPair([]byte(serverCert), []byte(serverKey))
			Expect(err).NotTo(HaveOccurred())

			loginATCServer.HTTPTestServer.TLS = &tls.Config{
				Certificates: []tls.Certificate{cert},
			}
			loginATCServer.HTTPTestServer.StartTLS()
		})

		AfterEach(func() {
			loginATCServer.Close()
		})

		Context("when already logged in with ca cert", func() {
			var caCertFilePath string

			BeforeEach(func() {
				loginATCServer.AppendHandlers(
					infoHandler(),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/some-team/auth/methods"),
						ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
					),
					tokenHandler("some-team"),
				)

				caCertFile, err := ioutil.TempFile("", "fly-login-test")
				Expect(err).NotTo(HaveOccurred())
				caCertFilePath = caCertFile.Name()

				err = ioutil.WriteFile(caCertFilePath, []byte(serverCert), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())

				setupFlyCmd := exec.Command(
					flyPath,
					"-t", "some-target",
					"login",
					"-c", loginATCServer.URL(),
					"-n", "some-team",
					"--ca-cert", caCertFilePath,
				)
				sess, err := gexec.Start(setupFlyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			})

			AfterEach(func() {
				os.RemoveAll(caCertFilePath)
			})

			Context("when ca cert is not provided", func() {
				It("is using saved ca cert", func() {
					loginATCServer.AppendHandlers(
						infoHandler(),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/some-team/auth/methods"),
							ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{}),
						),
						tokenHandler("some-team"),
					)

					flyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-n", "some-team")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))
				})
			})
		})
	})

	Describe("login", func() {
		var (
			flyCmd *exec.Cmd
			stdin  io.WriteCloser
		)

		BeforeEach(func() {
			loginATCServer = ghttp.NewServer()
			flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL())

			var err error
			stdin, err = flyCmd.StdinPipe()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			loginATCServer.Close()
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
				flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL())
				stdin, err = flyCmd.StdinPipe()
				Expect(err).NotTo(HaveOccurred())

				loginATCServer.AppendHandlers(
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
				loginATCServer.AppendHandlers(
					infoHandler(),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/auth/methods"),
						ghttp.RespondWithJSONEncoded(200, []atc.AuthMethod{
							{
								Type:        atc.AuthTypeBasic,
								DisplayName: "Basic",
								AuthURL:     "https://example.com/login/basic?team_name=main",
							},
							{
								Type:        atc.AuthTypeOAuth,
								DisplayName: "OAuth Type 1",
								AuthURL:     "https://example.com/auth/oauth-1?team_name=main",
							},
							{
								Type:        atc.AuthTypeOAuth,
								DisplayName: "OAuth Type 2",
								AuthURL:     "https://example.com/auth/oauth-2?team_name=main",
							},
						}),
					),
				)
			})

			Context("when an OAuth method is chosen", func() {
				It("logs into fly and uses the correct token", func() {
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

					r, _ := regexp.Compile("fly_local_port=(\\d+)")
					port := r.FindStringSubmatch(string(sess.Out.Contents()))[1]

					client := &http.Client{
						CheckRedirect: func(req *http.Request, via []*http.Request) error {
							return http.ErrUseLastResponse
						},
					}

					response, err := client.Get(fmt.Sprintf("http://localhost:%s/oauth/callback?token=Bearer%%20the-token", port))
					Expect(err).ToNot(HaveOccurred())
					Expect(response.StatusCode).To(Equal(http.StatusTemporaryRedirect))
					Expect(response.Header.Get("Location")).To(Equal(fmt.Sprintf("%s/public/fly_success", loginATCServer.URL())))

					Eventually(sess.Out).Should(gbytes.Say("target saved"))

					err = stdin.Close()
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))

					loginATCServer.AppendHandlers(
						infoHandler(),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines"),
							ghttp.VerifyHeaderKV("Authorization", "Bearer the-token"),
							ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
								{Name: "pipeline-1"},
							}),
						),
					)

					otherCmd := exec.Command(flyPath, "-t", "some-target", "pipelines")

					sess, err = gexec.Start(otherCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited

					Expect(sess).To(gbytes.Say("pipeline-1"))

					Expect(sess.ExitCode()).To(Equal(0))

				})
			})

			Context("when a Basic method is chosen", func() {
				BeforeEach(func() {
					loginATCServer.AppendHandlers(
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
						"login", "-c", loginATCServer.URL(),
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
							loginATCServer.AppendHandlers(
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
							loginATCServer.AppendHandlers(
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
				loginATCServer.AppendHandlers(
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
					"login", "-c", loginATCServer.URL(),
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
				loginATCServer.AppendHandlers(
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
				loginATCServer.AppendHandlers(
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
					loginATCServer.AppendHandlers(
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
				loginATCServer.AppendHandlers(
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
