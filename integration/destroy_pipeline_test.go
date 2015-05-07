package integration_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	var (
		flyPath   string
		atcServer *ghttp.Server
	)

	BeforeEach(func() {
		var err error

		flyPath, err = gexec.Build("github.com/concourse/fly")
		Ω(err).ShouldNot(HaveOccurred())
	})

	Describe("destroy-pipeline", func() {
		BeforeEach(func() {
			atcServer = ghttp.NewServer()
			os.Setenv("ATC_URL", atcServer.URL())
		})

		Context("when a pipeline name is not specified", func() {
			It("asks the user to specifiy a pipeline name", func() {
				flyCmd := exec.Command(flyPath, "destroy-pipeline")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))

				Ω(sess.Err).Should(gbytes.Say("you must specify a pipeline name"))
			})
		})

		Context("when a pipeline name is specified", func() {
			var (
				stdin io.Writer
				sess  *gexec.Session
			)

			JustBeforeEach(func() {
				var err error

				flyCmd := exec.Command(flyPath, "destroy-pipeline", "some-pipeline")
				stdin, err = flyCmd.StdinPipe()
				Ω(err).ShouldNot(HaveOccurred())

				sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Ω(err).ShouldNot(HaveOccurred())
				Eventually(sess).Should(gbytes.Say("!!! this will remove all data for pipeline `some-pipeline`"))
				Eventually(sess).Should(gbytes.Say(`are you sure\? \(y\/n\): `))
			})

			It("exits successfully if the user confirms", func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/api/v1/pipelines/some-pipeline"),
						ghttp.RespondWith(204, ""),
					),
				)

				fmt.Fprintln(stdin, "y")
				Eventually(sess).Should(gexec.Exit(0))
			})

			It("bails out if the user presses no", func() {
				fmt.Fprintln(stdin, "n")

				Eventually(sess).Should(gbytes.Say(`bailing out`))
				Eventually(sess).Should(gexec.Exit(1))
			})

			Context("and the pipeline exists", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/pipelines/some-pipeline"),
							ghttp.RespondWith(204, ""),
						),
					)
				})

				It("writes a success message to stdout", func() {
					fmt.Fprintln(stdin, "y")
					Eventually(sess).Should(gbytes.Say("`some-pipeline` deleted"))
					Eventually(sess).Should(gexec.Exit(0))
				})
			})

			Context("and the pipeline does not exist", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/pipelines/some-pipeline"),
							ghttp.RespondWith(404, ""),
						),
					)
				})

				It("writes an error message to stderr", func() {
					fmt.Fprintln(stdin, "y")
					Eventually(sess.Err).Should(gbytes.Say("`some-pipeline` does not exist"))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("and the api returns an internal server error", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/pipelines/some-pipeline"),
							ghttp.RespondWith(500, ""),
						),
					)
				})

				It("writes an error message to stderr", func() {
					fmt.Fprintln(stdin, "y")
					Eventually(sess.Err).Should(gbytes.Say("unexpected server error"))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("and the api returns an unexpected status code", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/pipelines/some-pipeline"),
							ghttp.RespondWith(402, ""),
						),
					)
				})

				It("writes an error message to stderr", func() {
					fmt.Fprintln(stdin, "y")
					Eventually(sess.Err).Should(gbytes.Say("unexpected response code: 402"))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})
		})
	})
})
