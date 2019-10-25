package integration_test

import (
	"os/exec"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

const second = 1
const minute = 60 * second
const hour = minute * 60
const day = hour * 24

var _ = Describe("Fly CLI", func() {
	Describe("workers", func() {
		var (
			flyCmd           *exec.Cmd
			worker1StartTime int64
			worker2StartTime int64
			worker3StartTime int64
			worker4StartTime int64
			worker5StartTime int64
			worker6StartTime int64
			worker7StartTime int64
		)

		BeforeEach(func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "workers")
		})

		Context("when workers are returned from the API", func() {
			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/workers"),
						ghttp.RespondWithJSONEncoded(200, []atc.Worker{
							{
								Name:             "worker-2",
								GardenAddr:       "1.2.3.4:7777",
								ActiveContainers: 0,
								ActiveTasks:      1,
								Platform:         "platform2",
								Tags:             []string{"tag2", "tag3"},
								ResourceTypes: []atc.WorkerResourceType{
									{Type: "resource-1", Image: "/images/resource-1"},
								},
								Team:      "team-1",
								State:     "running",
								Version:   "4.5.6",
								StartTime: worker2StartTime,
							},
							{
								Name:             "worker-6",
								GardenAddr:       "5.5.5.5:7777",
								ActiveContainers: 0,
								ActiveTasks:      1,
								Platform:         "platform2",
								Tags:             []string{"tag1"},
								Team:             "team-1",
								State:            "running",
								Version:          "1.2.3",
								Ephemeral:        true,
								StartTime:        worker6StartTime,
							},
							{
								Name:             "worker-7",
								GardenAddr:       "7.7.7.7:7777",
								ActiveContainers: 0,
								ActiveTasks:      0,
								Platform:         "platform2",
								Tags:             []string{"tag1"},
								Team:             "team-1",
								State:            "running",
								Version:          "",
								StartTime:        worker7StartTime,
							},
							{
								Name:             "worker-1",
								GardenAddr:       "2.2.3.4:7777",
								BaggageclaimURL:  "http://2.2.3.4:7788",
								ActiveContainers: 1,
								ActiveTasks:      1,
								Platform:         "platform1",
								Tags:             []string{"tag1"},
								ResourceTypes: []atc.WorkerResourceType{
									{Type: "resource-1", Image: "/images/resource-1"},
									{Type: "resource-2", Image: "/images/resource-2"},
								},
								Team:      "team-1",
								State:     "landing",
								Version:   "4.5.6",
								StartTime: worker1StartTime,
							},
							{
								Name:             "worker-3",
								GardenAddr:       "3.2.3.4:7777",
								ActiveContainers: 10,
								ActiveTasks:      1,
								Platform:         "platform3",
								Tags:             []string{},
								State:            "landed",
								Version:          "4.5.6",
								StartTime:        worker3StartTime,
							},
							{
								Name:             "worker-4",
								GardenAddr:       "",
								ActiveContainers: 7,
								ActiveTasks:      1,
								Platform:         "platform4",
								Tags:             []string{"tag1"},
								Team:             "team-1",
								State:            "stalled",
								Version:          "4.5.6",
								StartTime:        worker4StartTime,
							},
							{
								Name:             "worker-5",
								GardenAddr:       "3.2.3.4:7777",
								ActiveContainers: 5,
								ActiveTasks:      1,
								Platform:         "platform5",
								Tags:             []string{},
								State:            "retiring",
								Version:          "4.5.6",
								StartTime:        worker5StartTime,
							},
						}),
					),
				)
			})

			BeforeEach(func() {
				worker1StartTime = time.Now().Unix() - 2*day - 90*second
				worker2StartTime = time.Now().Unix() - 1*day - 90*second
				worker3StartTime = time.Now().Unix() - 10*hour - 3*minute - 50*second
				worker4StartTime = time.Now().Unix() - 8*hour - 30*minute - 50*second
				worker5StartTime = 0
				worker6StartTime = 0
				worker7StartTime = time.Now().Unix() + 700*second
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
						{Contents: "age", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "worker-1"}, {Contents: "1"}, {Contents: "platform1"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "landing"}, {Contents: "4.5.6"}, {Contents: "2d"}},
						{{Contents: "worker-2"}, {Contents: "0"}, {Contents: "platform2"}, {Contents: "tag2, tag3"}, {Contents: "team-1"}, {Contents: "running"}, {Contents: "4.5.6"}, {Contents: "1d"}},
						{{Contents: "worker-3"}, {Contents: "10"}, {Contents: "platform3"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "landed"}, {Contents: "4.5.6"}, {Contents: "10h3m"}},
						{{Contents: "worker-5"}, {Contents: "5"}, {Contents: "platform5"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "retiring"}, {Contents: "4.5.6"}, {Contents: "n/a", Color: color.New(color.Faint)}},
						{{Contents: "worker-6"}, {Contents: "0"}, {Contents: "platform2"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "running"}, {Contents: "1.2.3", Color: color.New(color.FgRed)}, {Contents: "n/a", Color: color.New(color.Faint)}},
						{{Contents: "worker-7"}, {Contents: "0"}, {Contents: "platform2"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "running"}, {Contents: "none", Color: color.New(color.FgRed)}, {Contents: "n/a", Color: color.New(color.Faint)}},
						{{Contents: "worker-4"}, {Contents: "7"}, {Contents: "platform4"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "stalled"}, {Contents: "4.5.6"}, {Contents: "8h30m"}},
					},
				}))
			})

			Context("when --json is given", func() {
				BeforeEach(func() {
					flyCmd.Args = append(flyCmd.Args, "--json")
					worker1StartTime = 0
					worker2StartTime = 0
					worker3StartTime = 0
					worker4StartTime = 0
					worker5StartTime = 0
					worker6StartTime = 0
					worker7StartTime = 0
				})

				It("prints response in json as stdout", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))
					Expect(sess.Out.Contents()).To(MatchJSON(`[
              {
                "addr": "1.2.3.4:7777",
                "baggageclaim_url": "",
                "active_containers": 0,
				"active_volumes": 0,
				"active_tasks": 1,
                "resource_types": [
                  {
                    "type": "resource-1",
                    "image": "/images/resource-1",
                    "version": "",
                    "privileged": false,
                    "unique_version_history": false
                  }
                ],
                "platform": "platform2",
                "tags": [
                  "tag2",
                  "tag3"
                ],
                "team": "team-1",
                "name": "worker-2",
                "version": "4.5.6",
                "start_time": 0,
                "state": "running",
                "ephemeral": false
              },
              {
                "addr": "5.5.5.5:7777",
                "baggageclaim_url": "",
                "active_containers": 0,
				"active_volumes": 0,
				"active_tasks": 1,
                "resource_types": null,
                "platform": "platform2",
                "tags": [
                  "tag1"
                ],
                "team": "team-1",
                "name": "worker-6",
                "version": "1.2.3",
                "start_time": 0,
                "state": "running",
                "ephemeral": true
              },
              {
                "addr": "7.7.7.7:7777",
                "baggageclaim_url": "",
                "active_containers": 0,
				"active_volumes": 0,
				"active_tasks": 0,
                "resource_types": null,
                "platform": "platform2",
                "tags": [
                  "tag1"
                ],
                "team": "team-1",
                "name": "worker-7",
                "version": "",
                "start_time": 0,
                "state": "running",
                "ephemeral": false
              },
              {
                "addr": "2.2.3.4:7777",
                "baggageclaim_url": "http://2.2.3.4:7788",
                "active_containers": 1,
				"active_volumes": 0,
				"active_tasks": 1,
                "resource_types": [
                  {
                    "type": "resource-1",
                    "image": "/images/resource-1",
                    "version": "",
                    "privileged": false,
                    "unique_version_history": false
                  },
                  {
                    "type": "resource-2",
                    "image": "/images/resource-2",
                    "version": "",
                    "privileged": false,
                    "unique_version_history": false
                  }
                ],
                "platform": "platform1",
                "tags": [
                  "tag1"
                ],
                "team": "team-1",
                "name": "worker-1",
                "version": "4.5.6",
                "start_time": 0,
                "state": "landing",
                "ephemeral": false
              },
              {
                "addr": "3.2.3.4:7777",
                "baggageclaim_url": "",
                "active_containers": 10,
				"active_volumes": 0,
				"active_tasks": 1,
                "resource_types": null,
                "platform": "platform3",
                "tags": [],
                "team": "",
                "name": "worker-3",
                "version": "4.5.6",
                "start_time": 0,
                "state": "landed",
                "ephemeral": false
              },
              {
                "addr": "",
                "baggageclaim_url": "",
                "active_containers": 7,
				"active_volumes": 0,
				"active_tasks": 1,
                "resource_types": null,
                "platform": "platform4",
                "tags": [
                  "tag1"
                ],
                "team": "team-1",
                "name": "worker-4",
                "version": "4.5.6",
                "start_time": 0,
                "state": "stalled",
                "ephemeral": false
              },
              {
                "addr": "3.2.3.4:7777",
                "baggageclaim_url": "",
                "active_containers": 5,
				"active_volumes": 0,
				"active_tasks": 1,
                "resource_types": null,
                "platform": "platform5",
                "tags": [],
                "team": "",
                "name": "worker-5",
                "version": "4.5.6",
                "start_time": 0,
                "state": "retiring",
                "ephemeral": false
              }
            ]`))
				})
			})

			Context("when --details is given", func() {
				BeforeEach(func() {
					flyCmd.Args = append(flyCmd.Args, "--details")
					worker1StartTime = 0
					worker2StartTime = 0
					worker3StartTime = 0
					worker4StartTime = 0
					worker5StartTime = 0
					worker6StartTime = 0
					worker7StartTime = 0
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
							{Contents: "version", Color: color.New(color.Bold)},
							{Contents: "age", Color: color.New(color.Bold)},
							{Contents: "garden address", Color: color.New(color.Bold)},
							{Contents: "baggageclaim url", Color: color.New(color.Bold)},
							{Contents: "active tasks", Color: color.New(color.Bold)},
							{Contents: "resource types", Color: color.New(color.Bold)},
						},
						Data: []ui.TableRow{
							{{Contents: "worker-1"}, {Contents: "1"}, {Contents: "platform1"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "landing"}, {Contents: "4.5.6"}, {Contents: "n/a", Color: color.New(color.Faint)}, {Contents: "2.2.3.4:7777"}, {Contents: "http://2.2.3.4:7788"}, {Contents: "1"}, {Contents: "resource-1, resource-2"}},
							{{Contents: "worker-2"}, {Contents: "0"}, {Contents: "platform2"}, {Contents: "tag2, tag3"}, {Contents: "team-1"}, {Contents: "running"}, {Contents: "4.5.6"}, {Contents: "n/a", Color: color.New(color.Faint)}, {Contents: "1.2.3.4:7777"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "1"}, {Contents: "resource-1"}},
							{{Contents: "worker-3"}, {Contents: "10"}, {Contents: "platform3"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "landed"}, {Contents: "4.5.6"}, {Contents: "n/a", Color: color.New(color.Faint)}, {Contents: "3.2.3.4:7777"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "1"}, {Contents: "none", Color: color.New(color.Faint)}},
							{{Contents: "worker-5"}, {Contents: "5"}, {Contents: "platform5"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "retiring"}, {Contents: "4.5.6"}, {Contents: "n/a", Color: color.New(color.Faint)}, {Contents: "3.2.3.4:7777"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "1"}, {Contents: "none", Color: color.New(color.Faint)}},
							{{Contents: "worker-6"}, {Contents: "0"}, {Contents: "platform2"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "running"}, {Contents: "1.2.3", Color: color.New(color.FgRed)}, {Contents: "n/a", Color: color.New(color.Faint)}, {Contents: "5.5.5.5:7777", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "1"}, {Contents: "none", Color: color.New(color.Faint)}},
							{{Contents: "worker-7"}, {Contents: "0"}, {Contents: "platform2"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "running"}, {Contents: "none", Color: color.New(color.FgRed)}, {Contents: "n/a", Color: color.New(color.Faint)}, {Contents: "7.7.7.7:7777", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "0"}, {Contents: "none", Color: color.New(color.Faint)}},
							{{Contents: "worker-4"}, {Contents: "7"}, {Contents: "platform4"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "stalled"}, {Contents: "4.5.6"}, {Contents: "n/a", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "1"}, {Contents: "none", Color: color.New(color.Faint)}},
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
								StartTime:        0,
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
								StartTime:        0,
							},
							{
								Name:             "worker-3",
								GardenAddr:       "3.2.3.4:7777",
								ActiveContainers: 5,
								Platform:         "platform3",
								Tags:             []string{},
								State:            "retiring",
								Version:          "4.5.6",
								StartTime:        0,
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
						{Contents: "age", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "worker-1"}, {Contents: "10"}, {Contents: "platform1"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "team-1"}, {Contents: "landing"}, {Contents: "4.5.6", Color: color.New(color.Faint)}, {Contents: "n/a", Color: color.New(color.Faint)}},
						{{Contents: "worker-2"}, {Contents: "0"}, {Contents: "platform2"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "running"}, {Contents: "4.5.6", Color: color.New(color.Faint)}, {Contents: "n/a", Color: color.New(color.Faint)}},
						{{Contents: "worker-3"}, {Contents: "5"}, {Contents: "platform3"}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "none", Color: color.New(color.Faint)}, {Contents: "retiring"}, {Contents: "4.5.6", Color: color.New(color.Faint)}, {Contents: "n/a", Color: color.New(color.Faint)}},
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
						{Contents: "age", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "worker-4"}, {Contents: "7"}, {Contents: "platform4"}, {Contents: "tag1"}, {Contents: "team-1"}, {Contents: "stalled"}, {Contents: "4.5.6"}, {Contents: "n/a", Color: color.New(color.Faint)}},
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
