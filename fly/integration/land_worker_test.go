package integration_test

import (
	"net/http"
	"os/exec"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("land-worker", func() {
	var flyCmd *exec.Cmd

	Context("when a worker name is specified", func() {
		BeforeEach(func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "land-worker", "-w", "worker-1")
		})

		Context("and the worker belongs to a team", func() {
			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/workers"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, []atc.Worker{
							{Name: "worker-1", Team: "team-a", State: "running"},
							{Name: "worker-2", Team: "team-b", State: "running"},
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/workers/worker-1/land"),
						ghttp.VerifyHeaderKV("Content-Type", "application/json"),
						ghttp.VerifyJSONRepresenting(atc.Worker{Team: "team-a"}),
						ghttp.RespondWith(http.StatusOK, nil),
					),
				)
			})

			It("lands the worker and forwards its team in the request body", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Eventually(sess.Out.Contents).Should(ContainSubstring("landed 'worker-1'\n"))

				Expect(atcServer.ReceivedRequests()).To(HaveLen(6),
					"login consumes first 4 requests. Last two requests are GET and PUT from land-worker")
			})
		})

		Context("and the worker is not in the list of workers", func() {
			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/workers"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, []atc.Worker{
							{Name: "worker-2", Team: "team-b", State: "running"},
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/workers/worker-1/land"),
						ghttp.VerifyJSONRepresenting(atc.Worker{Team: ""}),
						ghttp.RespondWith(http.StatusOK, nil),
					),
				)
			})

			It("lands the worker with an empty team", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Eventually(sess.Out.Contents).Should(ContainSubstring("landed 'worker-1'\n"))
			})
		})

		Context("and landing the worker fails", func() {
			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/workers"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, []atc.Worker{
							{Name: "worker-1", Team: "team-a", State: "running"},
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/workers/worker-1/land"),
						ghttp.RespondWith(http.StatusInternalServerError, nil),
					),
				)
			})

			It("exits with an error", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))
			})
		})

		Context("and listing the workers fails", func() {
			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/workers"),
						ghttp.RespondWith(http.StatusInternalServerError, nil),
					),
				)
			})

			It("exits with an error", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))
			})
		})
	})

	Context("when the worker name is not specified", func() {
		BeforeEach(func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "land-worker")
		})

		It("exits with an error about the required flag", func() {
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))
			Eventually(sess.Err).Should(gbytes.Say("worker"))
		})
	})
})
