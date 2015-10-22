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

	Describe("volumes", func() {
		var (
			args []string

			sess *gexec.Session
		)

		BeforeEach(func() {
			args = []string{}
			atcServer = ghttp.NewServer()
		})

		JustBeforeEach(func() {
			var err error

			flyCmd := exec.Command(flyPath, append([]string{"-t", atcServer.URL(), "volumes"}, args...)...)

			sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
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
								TTLInSeconds:      86340,
								ValidityInSeconds: 86400,
								WorkerName:        "dddddd",
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
						}),
					),
				)
			})

			It("lists them to the user, ordered by worker name and volume name", func() {
				Eventually(sess).Should(gbytes.Say("handle  ttl       validity  worker  version                     \n"))
				Eventually(sess).Should(gbytes.Say("aaabbb  01:23:20  01:40:00  cccccc  another: field, version: two\n"))
				Eventually(sess).Should(gbytes.Say("bbbbbb  00:00:50  00:10:00  cccccc  version: one                \n"))
				Eventually(sess).Should(gbytes.Say("aaaaaa  23:59:00  24:00:00  dddddd  version: three              \n"))
				Eventually(sess).Should(gbytes.Say("cccccc  00:03:20  00:05:00  dddddd  n/a                         \n"))
				Eventually(sess).Should(gexec.Exit(0))
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
				Eventually(sess.Err).Should(gbytes.Say("Unexpected Response"))
				Eventually(sess).Should(gexec.Exit(1))
			})
		})
	})
})
