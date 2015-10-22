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
	var (
		atcServer *ghttp.Server
	)

	Describe("containers", func() {
		var (
			flyCmd *exec.Cmd
		)

		BeforeEach(func() {
			atcServer = ghttp.NewServer()
			flyCmd = exec.Command(flyPath, "-t", atcServer.URL(), "containers")
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
				Expect(flyCmd).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "handle", Color: color.New(color.Bold)},
						{Contents: "name", Color: color.New(color.Bold)},
						{Contents: "pipeline", Color: color.New(color.Bold)},
						{Contents: "type", Color: color.New(color.Bold)},
						{Contents: "build id", Color: color.New(color.Bold)},
						{Contents: "worker", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "early-handle"}, {Contents: "git-repo"}, {Contents: "pipeline-name"}, {Contents: "get"}, {Contents: "123"}, {Contents: "worker-name-1"}},
						{{Contents: "handle-1"}, {Contents: "git-repo"}, {Contents: "pipeline-name"}, {Contents: "check"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "worker-name-1"}},
						{{Contents: "other-handle"}, {Contents: "unit-tests"}, {Contents: "pipeline-name"}, {Contents: "task"}, {Contents: "122"}, {Contents: "worker-name-2"}},
					},
				}))

				Expect(flyCmd).To(HaveExited(0))
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
				sess, err := gexec.Start(flyCmd, nil, nil)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess.Err).Should(gbytes.Say("Unexpected Response"))
				Eventually(sess).Should(gexec.Exit(1))
			})
		})
	})
})
