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
								ID:                "bbbbbb",
								WorkerName:        "cccccc",
								TTLInSeconds:      50,
								ValidityInSeconds: 600,
								ResourceVersion:   atc.Version{"version": "one"},
							},
							{
								ID:                "aaaaaa",
								WorkerName:        "dddddd",
								TTLInSeconds:      86340,
								ValidityInSeconds: 86400,
								ResourceVersion:   atc.Version{"version": "three"},
							},
							{
								ID:                "aaabbb",
								WorkerName:        "cccccc",
								TTLInSeconds:      5000,
								ValidityInSeconds: 6000,
								ResourceVersion:   atc.Version{"version": "two", "another": "field"},
							},
							{
								ID:                "cccccc",
								TTLInSeconds:      200,
								ValidityInSeconds: 300,
								WorkerName:        "dddddd",
							},
							{
								ID:                "eeeeee",
								TTLInSeconds:      0,
								ValidityInSeconds: 0,
								WorkerName:        "ffffff",
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
						{Contents: "ttl", Color: color.New(color.Bold)},
						{Contents: "validity", Color: color.New(color.Bold)},
						{Contents: "worker", Color: color.New(color.Bold)},
						{Contents: "version", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "aaabbb"}, {Contents: "01:23:20"}, {Contents: "01:40:00"}, {Contents: "cccccc"}, {Contents: "another: field, version: two"}},
						{{Contents: "bbbbbb"}, {Contents: "00:00:50"}, {Contents: "00:10:00"}, {Contents: "cccccc"}, {Contents: "version: one"}},
						{{Contents: "aaaaaa"}, {Contents: "23:59:00"}, {Contents: "24:00:00"}, {Contents: "dddddd"}, {Contents: "version: three"}},
						{{Contents: "cccccc"}, {Contents: "00:03:20"}, {Contents: "00:05:00"}, {Contents: "dddddd"}, {Contents: "n/a", Color: color.New(color.Faint)}},
						{{Contents: "eeeeee"}, {Contents: "indefinite"}, {Contents: "indefinite"}, {Contents: "ffffff"}, {Contents: "n/a", Color: color.New(color.Faint)}},
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
