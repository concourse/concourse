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
								WorkerName:   "worker-name-1",
								PipelineName: "pipeline-name",
								StepType:     "check",
								ResourceName: "git-repo",
							},
							{
								ID:           "early-handle",
								WorkerName:   "worker-name-1",
								PipelineName: "pipeline-name",
								JobName:      "job-name-1",
								BuildName:    "3",
								BuildID:      123,
								StepType:     "get",
								StepName:     "git-repo",
								Attempts:     []int{1, 5},
							},
							{
								ID:           "other-handle",
								WorkerName:   "worker-name-2",
								PipelineName: "pipeline-name",
								JobName:      "job-name-2",
								BuildName:    "2",
								BuildID:      122,
								StepType:     "task",
								StepName:     "unit-tests",
							},
							{
								ID:         "post-handle",
								WorkerName: "worker-name-3",
								BuildID:    142,
								StepType:   "task",
								StepName:   "one-off",
							},
						}),
					),
				)
			})

			It("lists them to the user, ordered by name", func() {
				Expect(flyCmd).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "handle", Color: color.New(color.Bold)},
						{Contents: "worker", Color: color.New(color.Bold)},
						{Contents: "pipeline", Color: color.New(color.Bold)},
						{Contents: "job", Color: color.New(color.Bold)},
						{Contents: "build #", Color: color.New(color.Bold)},
						{Contents: "build id", Color: color.New(color.Bold)},
						{Contents: "type", Color: color.New(color.Bold)},
						{Contents: "name", Color: color.New(color.Bold)},
						{Contents: "attempts", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "early-handle"}, {Contents: "worker-name-1"}, {Contents: "pipeline-name"}, {Contents: "job-name-1"}, {Contents: "3"}, {Contents: "123"}, {Contents: "get"}, {Contents: "git-repo"}, {Contents: "[1,5]"}},
						{{Contents: "handle-1"}, {Contents: "worker-name-1"}, {Contents: "pipeline-name"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "check"}, {Contents: "git-repo"}, {Contents: "n/a", Color: color.New(color.Faint)}},
						{{Contents: "other-handle"}, {Contents: "worker-name-2"}, {Contents: "pipeline-name"}, {Contents: "job-name-2"}, {Contents: "2"}, {Contents: "122"}, {Contents: "task"}, {Contents: "unit-tests"}, {Contents: "n/a", Color: color.New(color.Faint)}},
						{{Contents: "post-handle"}, {Contents: "worker-name-3"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "142"}, {Contents: "task"}, {Contents: "one-off"}, {Contents: "n/a", Color: color.New(color.Faint)}},
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
