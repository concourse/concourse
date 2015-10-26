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
	var atcServer *ghttp.Server

	Describe("pause-pipeline", func() {

		BeforeEach(func() {
			atcServer = ghttp.NewServer()
		})

		Context("when the pipeline name is specified", func() {
			var (
				path string
				err  error
			)
			BeforeEach(func() {
				path, err = atc.Routes.CreatePathForRoute(atc.PausePipeline, rata.Params{"pipeline_name": "awesome-pipeline"})
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

				It("pauses the pipeline", func() {
					flyCmd := exec.Command(flyPath, "-t", atcServer.URL(), "pause-pipeline", "-p", "awesome-pipeline")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`paused 'awesome-pipeline'`))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
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
					flyCmd := exec.Command(flyPath, "-t", atcServer.URL(), "pause-pipeline", "-p", "awesome-pipeline")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Err).Should(gbytes.Say(`pipeline 'awesome-pipeline' not found`))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
					Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				})
			})
		})
		Context("when the pipline name is not specified", func() {
			It("errors", func() {
				flyCmd := exec.Command(flyPath, "-t", atcServer.URL(), "pause-pipeline")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
				Expect(atcServer.ReceivedRequests()).To(HaveLen(0))
			})
		})
	})

})
