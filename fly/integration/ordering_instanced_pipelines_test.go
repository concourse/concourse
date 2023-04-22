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
	Describe("order-instanced-pipelines", func() {
		Context("when pipelines are specified", func() {
			var (
				path string
				err  error
			)
			BeforeEach(func() {
				path, err = atc.Routes.CreatePathForRoute(atc.OrderPipelinesWithinGroup, rata.Params{"team_name": "main", "pipeline_name": "awesome-pipeline"})
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the pipeline exists", func() {
				var instanceVars []atc.InstanceVars

				BeforeEach(func() {
					instanceVars = []atc.InstanceVars{
						{"branch": "main"},
						{"branch": "test"},
					}
				})

				JustBeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyJSONRepresenting(instanceVars),
							ghttp.VerifyRequest("PUT", path),
							ghttp.RespondWith(http.StatusOK, nil),
						),
					)
				})

				It("orders the instance pipelines", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "order-instanced-pipelines", "-g", "awesome-pipeline", "-p", "branch:main", "-p", "branch:test")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
						Eventually(sess).Should(gbytes.Say(`ordered instanced pipelines`))
						Eventually(sess).Should(gbytes.Say(`  - branch:main`))
						Eventually(sess).Should(gbytes.Say(`  - branch:test`))

					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})

				It("orders the instance pipeline with alias", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "oip", "-g", "awesome-pipeline", "-p", "branch:main", "-p", "branch:test")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
						Eventually(sess).Should(gbytes.Say(`ordered instanced pipelines`))
						Eventually(sess).Should(gbytes.Say(`  - branch:main`))
						Eventually(sess).Should(gbytes.Say(`  - branch:test`))

					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})
			})

			Context("when ordering fails", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", path),
							ghttp.RespondWith(http.StatusBadRequest, "pipeline 'awesome-pipeline/branch:main' not found"),
						),
					)
				})

				It("prints error message", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "order-instanced-pipelines", "-g", "awesome-pipeline", "-p", "branch:main")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(1))
						Eventually(sess.Err).Should(gbytes.Say(`failed to order instanced pipelines`))
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
					flyCmd := exec.Command(flyPath, "-t", targetName, "order-instanced-pipelines")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
					Expect(sess.Err).Should(gbytes.Say("error: the required flags `" + osFlag("g", "group") + "' and `" + osFlag("p", "pipeline") + "' were not specified"))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(0))
			})
		})
	})
})
