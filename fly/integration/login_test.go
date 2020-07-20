package integration_test

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/version"
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
				tokenHandler(),
				userInfoHandler(),
			)

			flyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "-u", "user", "-p", "pass")

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
					tokenHandler(),
					userInfoHandler(),
				)

				setupFlyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "-n", "some-team", "-u", "user", "-p", "pass")
				err := setupFlyCmd.Run()
				Expect(err).NotTo(HaveOccurred())
			})

			It("uses the saved team name", func() {
				loginATCServer.AppendHandlers(
					infoHandler(),
					tokenHandler(),
					userInfoHandler(),
				)

				flyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-u", "user", "-p", "pass")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gbytes.Say("logging in to team 'some-team'"))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			})
		})
	})

	Context("with no specified flag but extra arguments ", func() {

		BeforeEach(func() {
			loginATCServer = ghttp.NewServer()
		})

		AfterEach(func() {
			loginATCServer.Close()
		})

		It("return error indicating login failed with unknown arguments", func() {

			flyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "unknown-argument", "blah")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).NotTo(Equal(0))
			Expect(sess.Err).To(gbytes.Say(`unexpected argument \[unknown-argument, blah\]`))
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
				tokenHandler(),
				userInfoHandler(),
			)

			flyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "-n", "some-team", "-u", "user", "-p", "pass")

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
					tokenHandler(),
					userInfoHandler(),
				)

				flyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "-n", "some-team", "-u", "user", "-p", "pass")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

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
					tokenHandler(),
					userInfoHandler(),
				)

				flyCmd := exec.Command(flyPath, "--verbose", "-t", "some-target", "login", "-c", loginATCServer.URL(), "-n", "some-team", "-u", "user", "-p", "pass")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err).Should(gbytes.Say("HTTP/1.1 200 OK"))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			})
		})

		Context("when already logged in as different team", func() {
			BeforeEach(func() {
				loginATCServer.AppendHandlers(
					infoHandler(),
					tokenHandler(),
					userInfoHandler(),
				)

				setupFlyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "-n", "some-team", "-u", "user", "-p", "pass")
				err := setupFlyCmd.Run()
				Expect(err).NotTo(HaveOccurred())
			})

			It("passes provided team name", func() {
				loginATCServer.AppendHandlers(
					infoHandler(),
					tokenHandler(),
					userInfoHandler(),
				)

				flyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-n", "some-other-team", "-u", "user", "-p", "pass")

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
					tokenHandler(),
					userInfoHandler(),
				)

				caCertFile, err := ioutil.TempFile("", "fly-login-test")
				Expect(err).NotTo(HaveOccurred())
				caCertFilePath = caCertFile.Name()

				err = ioutil.WriteFile(caCertFilePath, []byte(serverCert), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())

				setupFlyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "-n", "some-team", "--ca-cert", caCertFilePath, "-u", "user", "-p", "pass")

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
						tokenHandler(),
						userInfoHandler(),
					)

					flyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-n", "some-team", "-u", "user", "-p", "pass")

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
		)

		BeforeEach(func() {
			loginATCServer = ghttp.NewServer()
		})

		AfterEach(func() {
			loginATCServer.Close()
		})

		Context("with authorization_code grant", func() {
			BeforeEach(func() {
				loginATCServer.AppendHandlers(
					infoHandler(),
					userInfoHandler(),
				)
			})

			It("allows providing the token via stdin", func() {
				flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL())

				stdin, err := flyCmd.StdinPipe()
				Expect(err).NotTo(HaveOccurred())

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("navigate to the following URL in your browser:"))
				Eventually(sess.Out).Should(gbytes.Say("http://127.0.0.1:(\\d+)/login\\?fly_port=(\\d+)"))
				Eventually(sess.Out).Should(gbytes.Say("or enter token manually"))

				_, err = fmt.Fprintf(stdin, "Bearer some-token\n")
				Expect(err).NotTo(HaveOccurred())

				err = stdin.Close()
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			})

			Context("when the token from stdin is malformed", func() {
				It("logs an error and accepts further input", func() {
					flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL())

					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("or enter token manually"))

					_, err = fmt.Fprintf(stdin, "not a token\n")
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("token must be of the format 'TYPE VALUE', e.g. 'Bearer ...'"))

					_, err = fmt.Fprintf(stdin, "Bearer ok-this-time-its-the-real-deal\n")
					Expect(err).NotTo(HaveOccurred())

					err = stdin.Close()
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))
				})
			})

			Context("when the token from stdin is terminated with an EOF", func() {
				It("accepts the input", func() {
					flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL())

					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("or enter token manually"))

					_, err = fmt.Fprintf(stdin, "bearer no-new-line-here")
					Expect(err).NotTo(HaveOccurred())

					err = stdin.Close()
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))
				})

				It("ignores empty input", func() {
					flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL())

					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					err = stdin.Close()
					Expect(err).NotTo(HaveOccurred())

					Consistently(sess.Out).ShouldNot(gbytes.Say("error"))

					sess.Kill()
				})
			})

			Context("token callback listener", func() {
				var resp *http.Response
				var req *http.Request
				var sess *gexec.Session

				BeforeEach(func() {
					flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL())
					_, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())
					sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess.Out).Should(gbytes.Say("or enter token manually"))
					scanner := bufio.NewScanner(bytes.NewBuffer(sess.Out.Contents()))
					var match []string
					for scanner.Scan() {
						re := regexp.MustCompile("fly_port=(\\d+)")
						match = re.FindStringSubmatch(scanner.Text())
						if len(match) > 0 {
							break
						}
					}
					flyPort := match[1]
					listenerURL := fmt.Sprintf("http://127.0.0.1:%s?token=Bearer%%20some-token", flyPort)
					req, err = http.NewRequest("GET", listenerURL, nil)
					Expect(err).NotTo(HaveOccurred())
				})

				JustBeforeEach(func() {
					loginATCServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/fly_success"),
						ghttp.RespondWith(200, ""),
					))
					client := &http.Client{
						CheckRedirect: func(req *http.Request, via []*http.Request) error {
							return http.ErrUseLastResponse
						},
					}
					var err error
					resp, err = client.Do(req)
					Expect(err).NotTo(HaveOccurred())
					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))
				})

				It("sets a CORS header for the ATC being logged in to", func() {
					corsHeader := resp.Header.Get("Access-Control-Allow-Origin")
					Expect(corsHeader).To(Equal(loginATCServer.URL()))
				})

				It("responds successfully", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))
				})

				Context("when the request comes from a human operating a browser", func() {
					BeforeEach(func() {
						req.Header.Add("Upgrade-Insecure-Requests", "1")
					})

					It("redirects back to noop fly success page", func() {
						Expect(resp.StatusCode).To(Equal(http.StatusFound))
						locationHeader := resp.Header.Get("Location")
						Expect(locationHeader).To(Equal(fmt.Sprintf("%s/fly_success?noop=true", loginATCServer.URL())))
					})
				})
			})
		})

		Context("with password grant", func() {
			BeforeEach(func() {
				credentials := base64.StdEncoding.EncodeToString([]byte("fly:Zmx5"))
				loginATCServer.AppendHandlers(
					infoHandler(),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/sky/issuer/token"),
						ghttp.VerifyHeaderKV("Content-Type", "application/x-www-form-urlencoded"),
						ghttp.VerifyHeaderKV("Authorization", fmt.Sprintf("Basic %s", credentials)),
						ghttp.VerifyFormKV("grant_type", "password"),
						ghttp.VerifyFormKV("username", "some_username"),
						ghttp.VerifyFormKV("password", "some_password"),
						ghttp.VerifyFormKV("scope", "openid profile email federated:id groups"),
						ghttp.RespondWithJSONEncoded(200, map[string]string{
							"token_type":   "Bearer",
							"access_token": "access-token",
						}),
					),
					userInfoHandler(),
				)
			})

			It("takes username and password as cli arguments", func() {
				flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "-u", "some_username", "-p", "some_password")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Consistently(sess.Out.Contents).ShouldNot(ContainSubstring("some_password"))

				Eventually(sess.Out).Should(gbytes.Say("target saved"))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			})

			Context("after logging in succeeds", func() {
				BeforeEach(func() {
					flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "-u", "some_username", "-p", "some_password")
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Consistently(sess.Out.Contents).ShouldNot(ContainSubstring("some_password"))

					Eventually(sess.Out).Should(gbytes.Say("target saved"))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))
				})

				It("flyrc is backwards-compatible with pre-v5.4.0", func() {
					flyRcContents, err := ioutil.ReadFile(homeDir + "/.flyrc")
					Expect(err).NotTo(HaveOccurred())
					Expect(string(flyRcContents)).To(HavePrefix("targets:"))
				})

				Describe("running other commands", func() {
					BeforeEach(func() {
						loginATCServer.AppendHandlers(
							infoHandler(),
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines"),
								ghttp.VerifyHeaderKV("Authorization", "Bearer access-token"),
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
						credentials := base64.StdEncoding.EncodeToString([]byte("fly:Zmx5"))

						loginATCServer.AppendHandlers(
							infoHandler(),
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("POST", "/sky/issuer/token"),
								ghttp.VerifyHeaderKV("Content-Type", "application/x-www-form-urlencoded"),
								ghttp.VerifyHeaderKV("Authorization", fmt.Sprintf("Basic %s", credentials)),
								ghttp.VerifyFormKV("grant_type", "password"),
								ghttp.VerifyFormKV("username", "some_other_user"),
								ghttp.VerifyFormKV("password", "some_other_pass"),
								ghttp.VerifyFormKV("scope", "openid profile email federated:id groups"),
								ghttp.RespondWithJSONEncoded(200, map[string]string{
									"token_type":   "Bearer",
									"access_token": "some-new-token",
								}),
							),
							userInfoHandler(),
							infoHandler(),
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines"),
								ghttp.VerifyHeaderKV("Authorization", "Bearer some-new-token"),
								ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
									{Name: "pipeline-2"},
								}),
							),
						)
					})

					It("updates the token", func() {
						loginAgainCmd := exec.Command(flyPath, "-t", "some-target", "login", "-u", "some_other_user", "-p", "some_other_pass")

						sess, err := gexec.Start(loginAgainCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Consistently(sess.Out.Contents).ShouldNot(ContainSubstring("some_other_pass"))

						Eventually(sess.Out).Should(gbytes.Say("target saved"))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))

						otherCmd := exec.Command(flyPath, "-t", "some-target", "pipelines")

						sess, err = gexec.Start(otherCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited

						Expect(sess).To(gbytes.Say("pipeline-2"))

						Expect(sess.ExitCode()).To(Equal(0))
					})
				})
			})
		})

		Context("when fly and atc differ in major versions", func() {
			var flyVersion string

			BeforeEach(func() {
				major, minor, patch, err := version.GetSemver(atcVersion)
				Expect(err).NotTo(HaveOccurred())

				flyVersion = fmt.Sprintf("%d.%d.%d", major+1, minor, patch)
				flyPath, err := gexec.Build(
					"github.com/concourse/concourse/fly",
					"-ldflags", fmt.Sprintf("-X github.com/concourse/concourse.Version=%s", flyVersion),
				)
				Expect(err).NotTo(HaveOccurred())
				flyCmd = exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "-u", "user", "-p", "pass")

				loginATCServer.AppendHandlers(
					infoHandler(),
					tokenHandler(),
					userInfoHandler(),
				)
			})

			It("warns user and does not fail", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Err).To(gbytes.Say(`fly version \(%s\) is out of sync with the target \(%s\). to sync up, run the following:\n\n    `, flyVersion, atcVersion))
				Expect(sess.Err).To(gbytes.Say(`fly.* -t some-target sync\n`))
			})
		})

		Context("cannot successfully login", func() {
			Context("team does not exist", func() {
				It("returns a warning", func() {
					loginATCServer.AppendHandlers(
						infoHandler(),
						tokenHandler(),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/user"),
							ghttp.RespondWithJSONEncoded(200, map[string]interface{}{
								"user_name": "user",
								"teams": map[string][]string{
									"other_team": {"owner"},
								},
							}),
						),
					)

					flyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "-n", "any-team", "-u", "user", "-p", "pass")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Err).Should(gbytes.Say("you are not a member of 'any-team' or the team does not exist"))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
				})
			})
			Context("/api/v1/user returns garbage", func() {
				It("returns a warning", func() {
					loginATCServer.AppendHandlers(
						infoHandler(),
						tokenHandler(),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/user"),
							ghttp.RespondWithJSONEncoded(200, map[string]interface{}{
								"a-key": "a-value",
							}),
						),
					)

					flyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "-n", "any-team", "-u", "user", "-p", "pass")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Err).Should(gbytes.Say("unable to verify role on team"))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
				})
			})
		})

		Context("when logging in as an admin user", func() {
			It("can login to any team", func() {
				loginATCServer.AppendHandlers(
					infoHandler(),
					tokenHandler(),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/user"),
						ghttp.RespondWithJSONEncoded(200, map[string]interface{}{
							"user_name": "admin_user",
							"is_admin":  true,
						}),
					),
				)

				flyCmd := exec.Command(flyPath, "-t", "some-target", "login", "-c", loginATCServer.URL(), "-n", "any-team", "-u", "admin_user", "-p", "pass")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("target saved"))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			})
		})
	})
})
