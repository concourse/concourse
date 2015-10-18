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

	Describe("containers", func() {
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

			flyCmd := exec.Command(flyPath, append([]string{"-t", atcServer.URL(), "containers"}, args...)...)

			sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when containers are returned from the API", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/containers"),
						ghttp.RespondWithJSONEncoded(200, []atc.Container{
							{
								ID:           "handle-1",
								PipelineName: "pipeline-name",
								Type:         "check",
								Name:         "git-repo",
								BuildID:      0,
								WorkerName:   "worker-name-1",
							},
							{
								ID:           "early-handle",
								PipelineName: "pipeline-name",
								Type:         "get",
								Name:         "git-repo",
								BuildID:      123,
								WorkerName:   "worker-name-1",
							},
							{
								ID:           "other-handle",
								PipelineName: "pipeline-name",
								Type:         "task",
								Name:         "unit-tests",
								BuildID:      122,
								WorkerName:   "worker-name-2",
							},
						}),
					),
				)
			})

			It("lists them to the user, ordered by name", func() {
				Eventually(sess).Should(gbytes.Say("handle        name        pipeline       type   build id  worker       \n"))
				Eventually(sess).Should(gbytes.Say("early-handle  git-repo    pipeline-name  get    123       worker-name-1\n"))
				Eventually(sess).Should(gbytes.Say("handle-1      git-repo    pipeline-name  check  none      worker-name-1\n"))
				Eventually(sess).Should(gbytes.Say("other-handle  unit-tests  pipeline-name  task   122       worker-name-2\n"))
				Eventually(sess).Should(gexec.Exit(0))
			})
		})

		Context("and the api returns an internal server error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/containers"),
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
