package integration_test

import (
	"fmt"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
	"io"
	"net/http"
	"os/exec"

	. "github.com/onsi/ginkgo"
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
				path string
				err  error
			)

			BeforeEach(func() {
				path, err = atc.Routes.CreatePathForRoute(atc.ArchivePipeline, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the pipeline exists", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", path),
							ghttp.RespondWith(http.StatusOK, nil),
						),
					)
				})

				Context("when the user confirms", func() {
					It("archives the pipeline", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "archive-pipeline", "-p", "awesome-pipeline")
							stdin, err := flyCmd.StdinPipe()
							Expect(err).NotTo(HaveOccurred())

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess).Should(gbytes.Say("archive pipeline 'awesome-pipeline'?"))
							yes(stdin)

							Eventually(sess).Should(gbytes.Say(`archived 'awesome-pipeline'`))

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
							flyCmd := exec.Command(flyPath, "-t", targetName, "archive-pipeline", "-p", "awesome-pipeline")
							stdin, err := flyCmd.StdinPipe()
							Expect(err).NotTo(HaveOccurred())

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess).Should(gbytes.Say("archive pipeline 'awesome-pipeline'?"))
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
							flyCmd := exec.Command(flyPath, "-t", targetName, "archive-pipeline", "-n", "-p", "awesome-pipeline")

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess).Should(gbytes.Say(`archived 'awesome-pipeline'`))

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

					Eventually(sess.Err).Should(gbytes.Say(`Either a pipeline name or --all are required`))

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

					Eventually(sess.Err).Should(gbytes.Say(`A pipeline and --all can not both be specified`))

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

		Context("when specifying a pipeline name with a '/' character in it", func() {
			It("fails and says '/' characters are not allowed", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "archive-pipeline", "-p", "forbidden/pipelinename")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say("error: pipeline name cannot contain '/'"))
			})
		})
	})
})
