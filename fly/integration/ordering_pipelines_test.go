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
	Describe("ordering-pipeline", func() {
		Context("when pipeline names are specified", func() {
			var (
				path string
				err  error
			)
			BeforeEach(func() {
				path, err = atc.Routes.CreatePathForRoute(atc.OrderPipelines, rata.Params{"team_name": "main"})
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the pipeline exists", func() {
				var requestBody atc.OrderPipelinesRequest

				BeforeEach(func() {
					requestBody = atc.OrderPipelinesRequest{
						{Name: "awesome-pipeline"},
						{Name: "awesome-pipeline-2"},
					}
				})

				JustBeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyJSONRepresenting(requestBody),
							ghttp.VerifyRequest("PUT", path),
							ghttp.RespondWith(http.StatusOK, nil),
						),
					)
				})

				Context("with instanced pipelines", func() {

					BeforeEach(func() {
						requestBody = atc.OrderPipelinesRequest{
							{Name: "awesome-pipeline", InstanceVars: atc.InstanceVars{"branch": "master"}},
							{Name: "awesome-pipeline", InstanceVars: atc.InstanceVars{"branch": "feature/bar"}},
						}
					})

					It("orders the pipeline instances", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "order-pipelines", "-p", "awesome-pipeline/branch:master", "-p", "awesome-pipeline/branch:feature/bar")

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(0))
							Eventually(sess).Should(gbytes.Say(`ordered pipelines`))
							Eventually(sess).Should(gbytes.Say(`  - awesome-pipeline/branch:master`))
							Eventually(sess).Should(gbytes.Say(`  - awesome-pipeline/branch:feature/bar`))

						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(2))
					})
				})

				It("orders the pipeline", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "order-pipelines", "-p", "awesome-pipeline", "-p", "awesome-pipeline-2")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
						Eventually(sess).Should(gbytes.Say(`ordered pipelines`))
						Eventually(sess).Should(gbytes.Say(`  - awesome-pipeline`))
						Eventually(sess).Should(gbytes.Say(`  - awesome-pipeline-2`))

					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})

				It("orders the pipeline with alias", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "op", "-p", "awesome-pipeline", "-p", "awesome-pipeline-2")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
						Eventually(sess).Should(gbytes.Say(`ordered pipelines`))
						Eventually(sess).Should(gbytes.Say(`  - awesome-pipeline`))
						Eventually(sess).Should(gbytes.Say(`  - awesome-pipeline-2`))

					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})
			})

			Context("when the alphabetical option is passed", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines"),
							ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
								{Name: "beautiful-pipeline", Paused: false, Public: false},
								{Name: "awesome-pipeline", Paused: true, Public: false},
								{Name: "delightful-pipeline", Paused: false, Public: true},
								{Name: "charming-pipeline", Paused: false, Public: true},
							}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", path),
							ghttp.RespondWith(http.StatusOK, nil),
						),
					)
				})

				It("orders all the pipelines in alphabetical order", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "order-pipelines", "--alphabetical")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
						Eventually(sess).Should(gbytes.Say(`ordered pipelines`))
						Eventually(sess).Should(gbytes.Say(`  - awesome-pipeline`))
						Eventually(sess).Should(gbytes.Say(`  - beautiful-pipeline`))
						Eventually(sess).Should(gbytes.Say(`  - charming-pipeline`))
						Eventually(sess).Should(gbytes.Say(`  - delightful-pipeline`))

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
							ghttp.RespondWith(http.StatusInternalServerError, nil),
						),
					)
				})

				It("prints error message", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "order-pipelines", "-p", "awesome-pipeline")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(1))
						Eventually(sess.Err).Should(gbytes.Say(`failed to order pipelines`))

					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})
			})
		})

		Context("when the pipline name is not specified", func() {
			It("errors", func() {
				Expect(func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "order-pipelines")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
					Expect(sess.Err).Should(gbytes.Say("error: either --pipeline or --alphabetical are required"))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(0))
			})
		})

		Context("when the pipeline flag is invalid", func() {
			It("fails and print invalid flag error", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "order-pipelines", "-p", "forbidden/pipelinename")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say("error: invalid argument for flag `" + osFlag("p", "pipeline")))
			})
		})

	})
})
