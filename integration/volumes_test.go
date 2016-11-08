package integration_test

import (
	"os/exec"

	"github.com/concourse/atc"
	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	Describe("volumes", func() {
		var (
			flyCmd *exec.Cmd
		)

		BeforeEach(func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "volumes")
		})

		Context("when volumes are returned from the API", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/volumes"),
						ghttp.RespondWithJSONEncoded(200, []atc.Volume{
							{
								ID:          "bbbbbb",
								WorkerName:  "cccccc",
								Type:        "copy",
								Identifier:  "some-parent-handle",
								SizeInBytes: 1024 * 1024,
							},
							{
								ID:          "aaaaaa",
								WorkerName:  "dddddd",
								Type:        "import",
								Identifier:  "path:version",
								SizeInBytes: 1741 * 1024,
							},
							{
								ID:          "aaabbb",
								WorkerName:  "cccccc",
								Type:        "output",
								Identifier:  "some-output",
								SizeInBytes: 4096 * 1024,
							},
							{
								ID:          "eeeeee",
								WorkerName:  "ffffff",
								Type:        "cow",
								Identifier:  "some-version",
								SizeInBytes: 8294 * 1024,
							},
							{
								ID:         "ihavenosize",
								WorkerName: "ffffff",
								Type:       "cow",
								Identifier: "some-version",
							},
						}),
					),
				)
			})

			It("lists them to the user, ordered by worker name and volume name", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))

				Expect(sess.Out).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "handle", Color: color.New(color.Bold)},
						{Contents: "worker", Color: color.New(color.Bold)},
						{Contents: "type", Color: color.New(color.Bold)},
						{Contents: "identifier", Color: color.New(color.Bold)},
						{Contents: "size", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "aaabbb"}, {Contents: "cccccc"}, {Contents: "output"}, {Contents: "some-output"}, {Contents: "4.0 MiB"}},
						{{Contents: "bbbbbb"}, {Contents: "cccccc"}, {Contents: "copy"}, {Contents: "some-parent-handle"}, {Contents: "1.0 MiB"}},
						{{Contents: "aaaaaa"}, {Contents: "dddddd"}, {Contents: "import"}, {Contents: "path:version"}, {Contents: "1.7 MiB"}},
						{{Contents: "eeeeee"}, {Contents: "ffffff"}, {Contents: "cow"}, {Contents: "some-version"}, {Contents: "8.1 MiB"}},
						{{Contents: "ihavenosize"}, {Contents: "ffffff"}, {Contents: "cow"}, {Contents: "some-version"}, {Contents: "unknown"}},
					},
				}))
			})
		})

		Context("and the api returns an internal server error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/volumes"),
						ghttp.RespondWith(500, ""),
					),
				)
			})

			It("writes an error message to stderr", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))
				Eventually(sess.Err).Should(gbytes.Say("Unexpected Response"))
			})
		})
	})
})
