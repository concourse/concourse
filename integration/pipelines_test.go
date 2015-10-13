package integration_test

import (
	"os/exec"

	"github.com/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	var (
		atcServer *ghttp.Server
	)

	Describe("pipelines", func() {
		var (
			sess *gexec.Session
		)

		BeforeEach(func() {
			atcServer = ghttp.NewServer()
		})

		JustBeforeEach(func() {
			var err error

			flyCmd := exec.Command(flyPath, "-t", atcServer.URL(), "pipelines")

			sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when pipelines are returned from the API", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/pipelines"),
						ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
							{Name: "pipeline-1-longer", URL: "/pipelines/pipeline-1", Paused: false},
							{Name: "pipeline-2", URL: "/pipelines/pipeline-2", Paused: true},
							{Name: "pipeline-3", URL: "/pipelines/pipeline-3", Paused: false},
						}),
					),
				)
			})

			It("lists them to the user", func() {
				Eventually(sess).Should(gbytes.Say("name               paused"))
				Eventually(sess).Should(gbytes.Say(``))
				Eventually(sess).Should(gbytes.Say(`pipeline-1-longer  no`))
				Eventually(sess).Should(gbytes.Say(`pipeline-2         yes`))
				Eventually(sess).Should(gbytes.Say(`pipeline-3         no`))
				Eventually(sess).Should(gexec.Exit(0))
			})
		})

		Context("and the api returns an internal server error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/pipelines"),
						ghttp.RespondWith(500, ""),
					),
				)
			})

			It("writes an error message to stderr", func() {
				Eventually(sess.Err).Should(gbytes.Say("unexpected server error"))
				Eventually(sess).Should(gexec.Exit(1))
			})
		})

		Context("and the api returns an unexpected status code", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/pipelines"),
						ghttp.RespondWith(402, ""),
					),
				)
			})

			It("writes an error message to stderr", func() {
				Eventually(sess.Err).Should(gbytes.Say("unexpected response code: 402"))
				Eventually(sess).Should(gexec.Exit(1))
			})
		})
	})
})
