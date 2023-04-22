package integration_test

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"

	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"
)

var _ = Describe("Fly CLI", func() {
	yes := func(stdin io.Writer) {
		fmt.Fprintf(stdin, "y\n")
	}

	no := func(stdin io.Writer) {
		fmt.Fprintf(stdin, "n\n")
	}

	Describe("archive-pipeline", func() {
		Context("when the pipeline name is specified", func() {
			var (
				path        string
				queryParams string
				err         error
			)

			BeforeEach(func() {
				path, err = atc.Routes.CreatePathForRoute(atc.ArchivePipeline, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
				Expect(err).NotTo(HaveOccurred())

				queryParams = "vars.branch=%22master%22"
			})

			Context("when the pipeline exists", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", path, queryParams),
							ghttp.RespondWith(http.StatusOK, nil),
						),
					)
				})

				Context("when the user confirms", func() {
					It("archives the pipeline", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "archive-pipeline", "-p", "awesome-pipeline/branch:master")
							stdin, err := flyCmd.StdinPipe()
							Expect(err).NotTo(HaveOccurred())

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess).Should(gbytes.Say("!!! archiving the pipeline will remove its configuration. Build history will be retained.\n\n"))
							Eventually(sess).Should(gbytes.Say("archive pipeline 'awesome-pipeline/branch:master'?"))
							yes(stdin)

							Eventually(sess).Should(gbytes.Say(`archived 'awesome-pipeline/branch:master'`))

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(0))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(2))
					})
				})

				Context("when the user declines", func() {
					It("does not archive the pipelines", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "archive-pipeline", "-p", "awesome-pipeline/branch:master")
							stdin, err := flyCmd.StdinPipe()
							Expect(err).NotTo(HaveOccurred())

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess).Should(gbytes.Say("archive pipeline 'awesome-pipeline/branch:master'?"))
							no(stdin)

							Eventually(sess).Should(gbytes.Say(`bailing out`))

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(0))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(1))
					})
				})

				Context("when running in non-interactive mode", func() {
					It("does not prompt the user", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "archive-pipeline", "-n", "-p", "awesome-pipeline/branch:master")

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess).Should(gbytes.Say(`archived 'awesome-pipeline/branch:master'`))

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(0))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(2))
					})
				})
			})

			Context("when the pipeline doesn't exist", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", path),
							ghttp.RespondWith(http.StatusNotFound, nil),
						),
					)
				})

				It("prints helpful message", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "archive-pipeline", "-n", "-p", "awesome-pipeline")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Err).Should(gbytes.Say(`pipeline 'awesome-pipeline' not found`))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(1))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})
			})
		})

		Context("when the pipeline name or --all is not specified", func() {
			It("errors", func() {
				Expect(func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "archive-pipeline")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Err).Should(gbytes.Say(`one of the flags '-p, --pipeline' or '-a, --all' is required`))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(0))
			})
		})

		Context("when both the pipeline name and --all are specified", func() {
			It("errors", func() {
				Expect(func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "archive-pipeline", "-p", "awesome-pipeline", "--all")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Err).Should(gbytes.Say(`only one of the flags '-p, --pipeline' or '-a, --all' is allowed`))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(0))
			})
		})

		Context("when the --all flag is passed, and there are unarchived pipelines", func() {
			var (
				somePath      string
				someOtherPath string
				err           error
			)

			BeforeEach(func() {
				somePath, err = atc.Routes.CreatePathForRoute(atc.ArchivePipeline, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
				Expect(err).NotTo(HaveOccurred())

				someOtherPath, err = atc.Routes.CreatePathForRoute(atc.ArchivePipeline, rata.Params{"pipeline_name": "more-awesome-pipeline", "team_name": "main"})
				Expect(err).NotTo(HaveOccurred())

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines"),
						ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
							{Name: "awesome-pipeline", Archived: false, Public: false},
							{Name: "more-awesome-pipeline", Archived: false, Public: false},
							{Name: "already-archived", Archived: true, Public: false},
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", somePath),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", someOtherPath),
						ghttp.RespondWith(http.StatusOK, nil),
					),
				)
			})

			Context("when the user confirms", func() {
				It("archives every currently unarchived pipeline", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "archive-pipeline", "--all")
						stdin, err := flyCmd.StdinPipe()
						Expect(err).NotTo(HaveOccurred())

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Out).Should(PrintTable(ui.Table{
							Headers: ui.TableRow{{Contents: "pipelines", Color: color.New(color.Bold)}},
							Data: []ui.TableRow{
								{{Contents: "awesome-pipeline"}},
								{{Contents: "more-awesome-pipeline"}},
							},
						}))

						Eventually(sess).Should(gbytes.Say("archive 2 pipelines?"))
						yes(stdin)

						Eventually(sess).Should(gbytes.Say(`archived 'awesome-pipeline'`))
						Eventually(sess).Should(gbytes.Say(`archived 'more-awesome-pipeline'`))
						Consistently(sess).ShouldNot(gbytes.Say(`archived 'already-archived'`))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(4))
				})
			})

			Context("when the user denies", func() {
				It("does not archive the pipelines", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "archive-pipeline", "--all")
						stdin, err := flyCmd.StdinPipe()
						Expect(err).NotTo(HaveOccurred())

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gbytes.Say("archive 2 pipelines?"))
						no(stdin)

						Eventually(sess).Should(gbytes.Say(`bailing out`))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})
			})

			Context("when running in non-interactive mode", func() {
				It("does not prompt the user", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "archive-pipeline", "-n", "--all")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gbytes.Say(`archived 'awesome-pipeline'`))
						Eventually(sess).Should(gbytes.Say(`archived 'more-awesome-pipeline'`))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(4))
				})
			})
		})

		Context("when the --all flag is passed, but there are no unarchived pipelines", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines"),
						ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
							{Name: "already-archived", Archived: true, Public: false},
						}),
					),
				)
			})

			It("prints a message and exits", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "archive-pipeline", "--all")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gbytes.Say("there are no unarchived pipelines"))
				Eventually(sess).Should(gbytes.Say("bailing out"))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			})
		})

		Context("when the pipeline flag is invalid", func() {
			It("fails and print invalid flag error", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "archive-pipeline", "-p", "forbidden/pipelinename")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say("error: invalid argument for flag `" + osFlag("p", "pipeline")))
			})
		})

		Context("user is NOT targeting the same team the pipeline belongs to", func() {
			team := "diff-team"
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", fmt.Sprintf("/api/v1/teams/%s", team)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{
							Name: team,
						}),
					),
				)
			})

			Context("when the pipeline name is specified", func() {
				var (
					path        string
					queryParams string
					err         error
				)

				BeforeEach(func() {
					path, err = atc.Routes.CreatePathForRoute(atc.ArchivePipeline, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": team})
					Expect(err).NotTo(HaveOccurred())

					queryParams = "vars.branch=%22master%22"
				})

				Context("when the pipeline exists", func() {
					BeforeEach(func() {
						atcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("PUT", path, queryParams),
								ghttp.RespondWith(http.StatusOK, nil),
							),
						)
					})
					It("archives the pipeline", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "archive-pipeline", "-p", "awesome-pipeline/branch:master", "--team", team)
							stdin, err := flyCmd.StdinPipe()
							Expect(err).NotTo(HaveOccurred())

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess).Should(gbytes.Say("!!! archiving the pipeline will remove its configuration. Build history will be retained.\n\n"))
							Eventually(sess).Should(gbytes.Say("archive pipeline 'awesome-pipeline/branch:master'?"))
							yes(stdin)

							Eventually(sess).Should(gbytes.Say(`archived 'awesome-pipeline/branch:master'`))

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(0))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(3))
					})
				})
				Context("when the pipeline doesn't exist", func() {
					BeforeEach(func() {
						atcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("PUT", path),
								ghttp.RespondWith(http.StatusNotFound, nil),
							),
						)
					})

					It("prints helpful message", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "archive-pipeline", "-n", "-p", "awesome-pipeline", "--team", team)

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess.Err).Should(gbytes.Say(`pipeline 'awesome-pipeline' not found`))

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(1))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(3))
					})
				})
			})
		})
	})
})
