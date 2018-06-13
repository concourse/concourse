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
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/volumes"),
						ghttp.RespondWithJSONEncoded(200, []atc.Volume{
							{
								ID:              "bbbbbb",
								WorkerName:      "cccccc",
								Type:            "container",
								ContainerHandle: "container-handle-b",
								Path:            "container-path-b",
							},
							{
								ID:         "aaaaaa",
								WorkerName: "dddddd",
								Type:       "resource",
								ResourceType: &atc.VolumeResourceType{
									ResourceType: &atc.VolumeResourceType{
										BaseResourceType: &atc.VolumeBaseResourceType{
											Name:    "base-resource-type",
											Version: "base-resource-version",
										},
										Version: atc.Version{"custom": "version"},
									},
									Version: atc.Version{"a": "b", "c": "d"},
								},
							},
							{
								ID:         "aaabbb",
								WorkerName: "cccccc",
								Type:       "resource-type",
								BaseResourceType: &atc.VolumeBaseResourceType{
									Name:    "base-resource-type",
									Version: "base-resource-version",
								},
							},
							{
								ID:              "eeeeee",
								WorkerName:      "ffffff",
								Type:            "container",
								ContainerHandle: "container-handle-e",
								Path:            "container-path-e",
							},
							{
								ID:              "ihavenosize",
								WorkerName:      "ffffff",
								Type:            "container",
								ContainerHandle: "container-handle-i",
								Path:            "container-path-i",
								ParentHandle:    "parent-handle-i",
							},
							{
								ID:           "task-cache-id",
								WorkerName:   "gggggg",
								Type:         "task-cache",
								PipelineName: "some-pipeline",
								JobName:      "some-job",
								StepName:     "some-step",
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
					},
					Data: []ui.TableRow{
						{
							{Contents: "aaabbb"},
							{Contents: "cccccc"},
							{Contents: "resource-type"},
							{Contents: "base-resource-type"},
						},
						{
							{Contents: "bbbbbb"},
							{Contents: "cccccc"},
							{Contents: "container"},
							{Contents: "container-handle-b"},
						},
						{
							{Contents: "aaaaaa"},
							{Contents: "dddddd"},
							{Contents: "resource"},
							{Contents: "a:b,c:d"},
						},
						{
							{Contents: "eeeeee"},
							{Contents: "ffffff"},
							{Contents: "container"},
							{Contents: "container-handle-e"},
						},
						{
							{Contents: "ihavenosize"},
							{Contents: "ffffff"},
							{Contents: "container"},
							{Contents: "container-handle-i"},
						},
						{
							{Contents: "task-cache-id"},
							{Contents: "gggggg"},
							{Contents: "task-cache"},
							{Contents: "some-pipeline/some-job/some-step"},
						},
					},
				}))
			})

			Context("when --json is given", func() {
				BeforeEach(func() {
					flyCmd.Args = append(flyCmd.Args, "--json")
				})

				It("prints response in json as stdout", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))
					Expect(sess.Out.Contents()).To(MatchJSON(`[
              {
                "id": "bbbbbb",
                "worker_name": "cccccc",
                "type": "container",
                "container_handle": "container-handle-b",
                "path": "container-path-b",
                "parent_handle": "",
                "resource_type": null,
                "base_resource_type": null,
                "pipeline_name": "",
                "job_name": "",
                "step_name": ""
              },
              {
                "id": "aaaaaa",
                "worker_name": "dddddd",
                "type": "resource",
                "container_handle": "",
                "path": "",
                "parent_handle": "",
                "resource_type": {
                  "resource_type": {
                    "resource_type": null,
                    "base_resource_type": {
                      "name": "base-resource-type",
                      "version": "base-resource-version"
                    },
                    "version": {
                      "custom": "version"
                    }
                  },
                  "base_resource_type": null,
                  "version": {
                    "a": "b",
                    "c": "d"
                  }
                },
                "base_resource_type": null,
                "pipeline_name": "",
                "job_name": "",
                "step_name": ""
              },
              {
                "id": "aaabbb",
                "worker_name": "cccccc",
                "type": "resource-type",
                "container_handle": "",
                "path": "",
                "parent_handle": "",
                "resource_type": null,
                "base_resource_type": {
                  "name": "base-resource-type",
                  "version": "base-resource-version"
                },
                "pipeline_name": "",
                "job_name": "",
                "step_name": ""
              },
              {
                "id": "eeeeee",
                "worker_name": "ffffff",
                "type": "container",
                "container_handle": "container-handle-e",
                "path": "container-path-e",
                "parent_handle": "",
                "resource_type": null,
                "base_resource_type": null,
                "pipeline_name": "",
                "job_name": "",
                "step_name": ""
              },
              {
                "id": "ihavenosize",
                "worker_name": "ffffff",
                "type": "container",
                "container_handle": "container-handle-i",
                "path": "container-path-i",
                "parent_handle": "parent-handle-i",
                "resource_type": null,
                "base_resource_type": null,
                "pipeline_name": "",
                "job_name": "",
                "step_name": ""
              },
              {
                "id": "task-cache-id",
                "worker_name": "gggggg",
                "type": "task-cache",
                "container_handle": "",
                "path": "",
                "parent_handle": "",
                "resource_type": null,
                "base_resource_type": null,
                "pipeline_name": "some-pipeline",
                "job_name": "some-job",
                "step_name": "some-step"
              }
            ]`))
				})
			})

			Context("when --details flag is set", func() {
				BeforeEach(func() {
					flyCmd = exec.Command(flyPath, "-t", targetName, "volumes", "--details")
				})

				It("displays detailed identifiers", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))

					Expect(sess.Out).To(PrintTable(ui.Table{
						Headers: ui.TableRow{
							{Contents: "handle", Color: color.New(color.Bold)},
							{Contents: "worker", Color: color.New(color.Bold)},
							{Contents: "type", Color: color.New(color.Bold)},
							{Contents: "identifier", Color: color.New(color.Bold)},
						},
						Data: []ui.TableRow{
							{
								{Contents: "aaabbb"},
								{Contents: "cccccc"},
								{Contents: "resource-type"},
								{Contents: "name:base-resource-type,version:base-resource-version"},
							},
							{
								{Contents: "bbbbbb"},
								{Contents: "cccccc"},
								{Contents: "container"},
								{Contents: "container:container-handle-b,path:container-path-b"},
							},
							{
								{Contents: "aaaaaa"},
								{Contents: "dddddd"},
								{Contents: "resource"},
								{Contents: "type:resource(name:base-resource-type,version:base-resource-version),version:a:b,c:d"},
							},
							{
								{Contents: "eeeeee"},
								{Contents: "ffffff"},
								{Contents: "container"},
								{Contents: "container:container-handle-e,path:container-path-e"},
							},
							{
								{Contents: "ihavenosize"},
								{Contents: "ffffff"},
								{Contents: "container"},
								{Contents: "container:container-handle-i,path:container-path-i,parent:parent-handle-i"},
							},
							{
								{Contents: "task-cache-id"},
								{Contents: "gggggg"},
								{Contents: "task-cache"},
								{Contents: "some-pipeline/some-job/some-step"},
							},
						},
					}))
				})
			})
		})

		Context("and the api returns an internal server error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/volumes"),
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
