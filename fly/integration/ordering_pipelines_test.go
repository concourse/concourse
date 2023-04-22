package integration_test

import (
	"net/http"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
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
				var pipelineNames []string

				BeforeEach(func() {
					pipelineNames = []string{
						"awesome-pipeline",
						"awesome-pipeline-2",
					}
				})

				JustBeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyJSONRepresenting(pipelineNames),
							ghttp.VerifyRequest("PUT", path),
							ghttp.RespondWith(http.StatusOK, nil),
						),
					)
				})

				It("orders the pipelines", func() {
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
								{Name: "awesome-pipeline", InstanceVars: map[string]interface{}{"hello": "world"}, Paused: true, Public: false},
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
						// Check that it dedupes pipeline names
						Consistently(sess).ShouldNot(gbytes.Say(`  - awesome-pipeline`))
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
							ghttp.RespondWith(http.StatusBadRequest, "pipeline 'awsome-pipeline' not found"),
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

						Consistently(sess.Err).ShouldNot(gbytes.Say(`Unexpected Response`))

					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})
			})
		})

		Context("when the pipeline name is not specified", func() {
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

				Expect(sess.Err).To(gbytes.Say(`error: pipeline name "forbidden/pipelinename" cannot contain '/'`))
			})
		})
	})
})
