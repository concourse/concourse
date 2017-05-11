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
	Describe("workers", func() {
		var (
			flyCmd *exec.Cmd
		)

		BeforeEach(func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "workers")
		})

		Context("when workers are returned from the API", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/workers"),
						ghttp.RespondWithJSONEncoded(200, []atc.Worker{
							{
								Name:             "worker-2",
								GardenAddr:       "1.2.3.4:7777",
								ActiveContainers: 0,
								Platform:         "platform2",
								Tags:             []string{"tag2", "tag3"},
								ResourceTypes: []atc.WorkerResourceType{
									{Type: "resource-1", Image: "/images/resource-1"},
								},
								Team:    "team-1",
								State:   "running",
								Version: "4.5.6",
							},
							{
								Name:             "worker-6",
								GardenAddr:       "5.5.5.5:7777",
								ActiveContainers: 0,
								Platform:         "platform2",
								Tags:             []string{"tag1"},
								Team:             "team-1",
								State:            "running",
								Version:          "1.2.3",
							},
							{
								Name:             "worker-7",
								GardenAddr:       "7.7.7.7:7777",
								ActiveContainers: 0,
								Platform:         "platform2",
								Tags:             []string{"tag1"},
								Team:             "team-1",
								State:            "running",
								Version:          "",
							},
							{
								Name:             "worker-1",
								GardenAddr:       "2.2.3.4:7777",
								BaggageclaimURL:  "http://2.2.3.4:7788",
								ActiveContainers: 1,
								Platform:         "platform1",
								Tags:             []string{"tag1"},
								ResourceTypes: []atc.WorkerResourceType{
									{Type: "resource-1", Image: "/images/resource-1"},
									{Type: "resource-2", Image: "/images/resource-2"},
								},
								Team:    "team-1",
								State:   "landing",
								Version: "4.5.6",
							},
							{
								Name:             "worker-3",
								GardenAddr:       "3.2.3.4:7777",
								ActiveContainers: 10,
								Platform:         "platform3",
								Tags:             []string{},
								State:            "landed",
								Version:          "4.5.6",
							},
							{
								Name:             "worker-4",
								GardenAddr:       "",
								ActiveContainers: 7,
								Platform:         "platform4",
								Tags:             []string{"tag1"},
								Team:             "team-1",
								State:            "stalled",
								Version:          "4.5.6",
							},
							{
								Name:             "worker-5",
								GardenAddr:       "3.2.3.4:7777",
								ActiveContainers: 5,
								Platform:         "platform5",
								Tags:             []string{},
								State:            "retiring",
								Version:          "4.5.6",
							},
						}),
					),
				)
			})

			It("lists them to the user, ordered by name, with outdated and stalled workers grouped together", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Out).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "name", Color: color.New(color.Bold)},
						{Contents: "containers", Color: color.New(color.Bold)},
						{Contents: "platform", Color: color.New(color.Bold)},
						{Contents: "tags", Color: color.New(color.Bold)},
						{Contents: "team", Color: color.New(color.Bold)},
						{Contents: "state", Color: color.New(color.Bold)},
						{Contents: "version", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "worker-1"}, {Contents: "1"}, {Contents: "platform1"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "landing"}, {Contents: "4.5.6"}},
						{{Contents: "worker-2"}, {Contents: "0"}, {Contents: "platform2"}, {Contents: "tag2, tag3"}, {Contents: "team-1"}, {Contents: "running"}, {Contents: "4.5.6"}},
						{{Contents: "worker-3"}, {Contents: "10"}, {Contents: "platform3"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "landed"}, {Contents: "4.5.6"}},
						{{Contents: "worker-5"}, {Contents: "5"}, {Contents: "platform5"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "retiring"}, {Contents: "4.5.6"}},
						{{Contents: "worker-6"}, {Contents: "0"}, {Contents: "platform2"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "running"}, {Contents: "1.2.3", Color: color.New(color.FgRed)}},
						{{Contents: "worker-7"}, {Contents: "0"}, {Contents: "platform2"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "running"}, {Contents: "none", Color: color.New(color.FgRed)}},
						{{Contents: "worker-4"}, {Contents: "7"}, {Contents: "platform4"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "stalled"}, {Contents: "4.5.6"}},
					},
				}))
			})

			Context("when --details is given", func() {
				BeforeEach(func() {
					flyCmd.Args = append(flyCmd.Args, "--details")
				})

				It("lists them to the user, ordered by name", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))
					Expect(sess.Out).To(PrintTable(ui.Table{
						Headers: ui.TableRow{
							{Contents: "name", Color: color.New(color.Bold)},
							{Contents: "containers", Color: color.New(color.Bold)},
							{Contents: "platform", Color: color.New(color.Bold)},
							{Contents: "tags", Color: color.New(color.Bold)},
							{Contents: "team", Color: color.New(color.Bold)},
							{Contents: "state", Color: color.New(color.Bold)},
							{Contents: "garden address", Color: color.New(color.Bold)},
							{Contents: "baggageclaim url", Color: color.New(color.Bold)},
							{Contents: "resource types", Color: color.New(color.Bold)},
						},
						Data: []ui.TableRow{
							{{Contents: "worker-1"}, {Contents: "1"}, {Contents: "platform1"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "landing"}, {Contents: "4.5.6"}, {Contents: "2.2.3.4:7777"}, {Contents: "http://2.2.3.4:7788"}, {Contents: "resource-1, resource-2"}},
							{{Contents: "worker-2"}, {Contents: "0"}, {Contents: "platform2"}, {Contents: "tag2, tag3"}, {Contents: "team-1"}, {Contents: "running"}, {Contents: "4.5.6"}, {Contents: "1.2.3.4:7777"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "resource-1"}},
							{{Contents: "worker-3"}, {Contents: "10"}, {Contents: "platform3"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "landed"}, {Contents: "4.5.6"}, {Contents: "3.2.3.4:7777"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}},
							{{Contents: "worker-5"}, {Contents: "5"}, {Contents: "platform5"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "retiring"}, {Contents: "4.5.6"}, {Contents: "3.2.3.4:7777"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}},
							{{Contents: "worker-6"}, {Contents: "0"}, {Contents: "platform2"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "running"}, {Contents: "1.2.3", Color: color.New(color.FgRed)}, {Contents: "5.5.5.5:7777", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}},
							{{Contents: "worker-7"}, {Contents: "0"}, {Contents: "platform2"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "running"}, {Contents: "none", Color: color.New(color.FgRed)}, {Contents: "7.7.7.7:7777", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}},
							{{Contents: "worker-4"}, {Contents: "7"}, {Contents: "platform4"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "stalled"}, {Contents: "4.5.6"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}},
						},
					}))
				})
			})
		})

		Context("when API does not return stalled or outdated workers", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/workers"),
						ghttp.RespondWithJSONEncoded(200, []atc.Worker{
							{
								Name:             "worker-2",
								GardenAddr:       "1.2.3.4:7777",
								ActiveContainers: 0,
								Platform:         "platform2",
								Tags:             []string{"tag1"},
								Team:             "team-1",
								State:            "running",
								Version:          "4.5.6",
							},
							{
								Name:             "worker-1",
								GardenAddr:       "3.2.3.4:7777",
								ActiveContainers: 10,
								Platform:         "platform1",
								Tags:             []string{},
								Team:             "team-1",
								State:            "landing",
								Version:          "4.5.6",
							},
							{
								Name:             "worker-3",
								GardenAddr:       "3.2.3.4:7777",
								ActiveContainers: 5,
								Platform:         "platform3",
								Tags:             []string{},
								State:            "retiring",
								Version:          "4.5.6",
							},
						}),
					),
				)
			})

			It("does not print second table", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Out).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "name", Color: color.New(color.Bold)},
						{Contents: "containers", Color: color.New(color.Bold)},
						{Contents: "platform", Color: color.New(color.Bold)},
						{Contents: "tags", Color: color.New(color.Bold)},
						{Contents: "team", Color: color.New(color.Bold)},
						{Contents: "state", Color: color.New(color.Bold)},
						{Contents: "version", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "worker-1"}, {Contents: "10"}, {Contents: "platform1"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "team-1"}, {Contents: "landing"}, {Contents: "4.5.6", Color: color.New(color.Faint)}},
						{{Contents: "worker-2"}, {Contents: "0"}, {Contents: "platform2"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "running"}, {Contents: "4.5.6", Color: color.New(color.Faint)}},
						{{Contents: "worker-3"}, {Contents: "5"}, {Contents: "platform3"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "retiring"}, {Contents: "4.5.6", Color: color.New(color.Faint)}},
					},
				}))
				Expect(sess.Out).NotTo(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "name", Color: color.New(color.Bold)},
						{Contents: "containers", Color: color.New(color.Bold)},
						{Contents: "platform", Color: color.New(color.Bold)},
						{Contents: "tags", Color: color.New(color.Bold)},
						{Contents: "team", Color: color.New(color.Bold)},
						{Contents: "state", Color: color.New(color.Bold)},
						{Contents: "version", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "worker-4"}, {Contents: "7"}, {Contents: "platform4"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "stalled"}, {Contents: "4.5.6"}},
					},
				}))
			})
		})

		Context("and the api returns an internal server error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/workers"),
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
