package integration_test

import (
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
	Describe("pause-pipeline", func() {
		Context("when the pipeline name is specified", func() {
			var (
				path        string
				queryParams string
				err         error
			)
			BeforeEach(func() {
				path, err = atc.Routes.CreatePathForRoute(atc.PausePipeline, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": teamName})
				Expect(err).NotTo(HaveOccurred())

				queryParams = "instance_vars=%7B%22branch%22%3A%22master%22%7D"
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

				It("pauses the pipeline", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "pause-pipeline", "-p", "awesome-pipeline/branch:master")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gbytes.Say(`paused 'awesome-pipeline/branch:master'`))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
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
						flyCmd := exec.Command(flyPath, "-t", targetName, "pause-pipeline", "-p", "awesome-pipeline")

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

		Context("when both the team and pipeline name are specified", func() {
			teamName := "other-team"
			var (
				path string
				err  error
			)
			BeforeEach(func() {
				path, err = atc.Routes.CreatePathForRoute(atc.PausePipeline, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": teamName})
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the pipeline exists", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/"+teamName),
							ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{
								Name: teamName,
							}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", path),
							ghttp.RespondWith(http.StatusOK, nil),
						),
					)
				})

				It("pauses the pipeline", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "pause-pipeline", "--team", teamName, "-p", "awesome-pipeline")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gbytes.Say(`paused 'awesome-pipeline'`))

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
							ghttp.VerifyRequest("GET", "/api/v1/teams/"+teamName),
							ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{
								Name: teamName,
							}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", path),
							ghttp.RespondWith(http.StatusNotFound, nil),
						),
					)
				})

				It("prints helpful message", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "pause-pipeline", "--team", teamName, "-p", "awesome-pipeline")

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

		Context("when the pipline name or --all is not specified", func() {
			It("errors", func() {
				Expect(func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "pause-pipeline")

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

		Context("when both the pipline name and --all are specified", func() {
			It("errors", func() {
				Expect(func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "pause-pipeline", "-p", "awesome-pipeline", "--all")

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

		Context("when the pipeline flag is invalid", func() {
			It("fails and print invalid flag error", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "pause-pipeline", "-p", "forbidden/pipelinename")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say("error: invalid argument for flag `" + osFlag("p", "pipeline")))
			})
		})

		Context("when the --all flag is passed", func() {
			var (
				somePath      string
				someOtherPath string
				err           error
			)

			BeforeEach(func() {
				somePath, err = atc.Routes.CreatePathForRoute(atc.PausePipeline, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": teamName})
				Expect(err).NotTo(HaveOccurred())

				someOtherPath, err = atc.Routes.CreatePathForRoute(atc.PausePipeline, rata.Params{"pipeline_name": "more-awesome-pipeline", "team_name": teamName})
				Expect(err).NotTo(HaveOccurred())

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines"),
						ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
							{Name: "awesome-pipeline", Paused: false, Public: false},
							{Name: "more-awesome-pipeline", Paused: true, Public: false},
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

			It("pauses every pipeline", func() {
				Expect(func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "pause-pipeline", "--all")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`paused 'awesome-pipeline'`))
					Eventually(sess).Should(gbytes.Say(`paused 'more-awesome-pipeline'`))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(4))
			})
		})
	})
})
