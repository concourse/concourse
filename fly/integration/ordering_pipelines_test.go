package integration_test

import (
	"net/http"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
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
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyJSONRepresenting([]string{"awesome-pipeline", "awesome-pipeline-2"}),
							ghttp.VerifyRequest("PUT", path),
							ghttp.RespondWith(http.StatusOK, nil),
						),
					)
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
					Expect(sess.Err).Should(gbytes.Say("error: the required flag `" + osFlag("p", "pipeline") + "' was not specified"))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(0))
			})
		})

		Context("when specifying a pipeline name with a '/' character in it", func() {
			It("fails and says '/' characters are not allowed", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "order-pipelines", "-p", "forbidden/pipelinename")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say("error: pipeline name cannot contain '/'"))
			})
		})

	})
})
