package integration_test

import (
	"os/exec"
	"time"

	"github.com/concourse/concourse/v8/atc"
	"github.com/concourse/concourse/v8/fly/rc"
	"github.com/concourse/concourse/v8/fly/ui"
	"github.com/fatih/color"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("fly health", func() {
	var (
		flyCmd        *exec.Cmd
		healthPayload atc.Health
	)

	BeforeEach(func() {
		atcServer.Reset()

		flyCmd = exec.Command(flyPath, "-t", targetName, "health")
		healthPayload = atc.Health{
			Status:    atc.HealthStatusOK,
			Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Database:  atc.DatabaseHealth{Status: atc.HealthStatusHealthy},
			Workers: atc.WorkerHealth{
				Status:  string(atc.HealthStatusHealthy),
				Total:   2,
				Running: 2,
			},
			Components: []atc.ComponentHealth{
				{
					Name:    atc.ComponentScheduler,
					Status:  atc.HealthStatusHealthy,
					Paused:  false,
					LastRan: time.Date(2024, 1, 1, 11, 59, 0, 0, time.UTC),
				},
			},
		}
	})

	Context("when the cluster is healthy", func() {
		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/health"),
					ghttp.RespondWithJSONEncoded(200, healthPayload),
				),
			)
		})

		It("prints a table with the cluster health and exits 0", func() {
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(PrintTable(ui.Table{
				Headers: ui.TableRow{
					{Contents: "subsystem", Color: color.New(color.Bold)},
					{Contents: "status", Color: color.New(color.Bold)},
					{Contents: "detail", Color: color.New(color.Bold)},
				},
				Data: []ui.TableRow{
					{{Contents: "overall"}, {Contents: "ok", Color: ui.SucceededColor}, {Contents: "2024-01-01T12:00:00Z"}},
					{{Contents: "database"}, {Contents: "healthy", Color: ui.SucceededColor}, {Contents: ""}},
					{{Contents: "workers"}, {Contents: "healthy", Color: ui.SucceededColor}, {Contents: "2/2 running"}},
					{{Contents: "scheduler"}, {Contents: "healthy", Color: ui.SucceededColor}, {Contents: "last ran: 2024-01-01T11:59:00Z"}},
				},
			}))
		})

		Context("when the token is expired or missing", func() {
			BeforeEach(func() {
				createFlyRc(rc.Targets{
					targetName: {
						API:      atcServer.URL(),
						TeamName: teamName,
						Token:    &rc.TargetToken{Type: "Bearer", Value: validAccessToken(time.Now().Add(-time.Hour))},
					},
				})
			})

			It("succeeds without requiring a valid token", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Out).To(gbytes.Say("ok"))

				Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
				Expect(atcServer.ReceivedRequests()[0].Header.Get("Authorization")).To(BeEmpty())
			})
		})

		Context("when --json is given", func() {
			BeforeEach(func() {
				flyCmd.Args = append(flyCmd.Args, "--json")
			})

			It("prints the response as JSON and exits 0", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Out.Contents()).To(MatchJSON(`{
					"status": "ok",
					"timestamp": "2024-01-01T12:00:00Z",
					"database": {"status": "healthy"},
					"workers": {"status": "healthy", "total": 2, "running": 2},
					"components": [{"name": "scheduler", "status": "healthy", "paused": false, "last_ran": "2024-01-01T11:59:00Z"}]
				}`))
			})
		})

		Context("when a component is paused and has previously run", func() {
			BeforeEach(func() {
				healthPayload.Components = []atc.ComponentHealth{
					{
						Name:    atc.ComponentScheduler,
						Status:  atc.HealthStatusHealthy,
						Paused:  true,
						LastRan: time.Date(2024, 1, 1, 11, 59, 0, 0, time.UTC),
					},
				}
			})

			It("shows both paused and last ran in the detail column", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Out).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "subsystem", Color: color.New(color.Bold)},
						{Contents: "status", Color: color.New(color.Bold)},
						{Contents: "detail", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "overall"}, {Contents: "ok", Color: ui.SucceededColor}, {Contents: "2024-01-01T12:00:00Z"}},
						{{Contents: "database"}, {Contents: "healthy", Color: ui.SucceededColor}, {Contents: ""}},
						{{Contents: "workers"}, {Contents: "healthy", Color: ui.SucceededColor}, {Contents: "2/2 running"}},
						{{Contents: "scheduler"}, {Contents: "healthy", Color: ui.SucceededColor}, {Contents: "paused, last ran: 2024-01-01T11:59:00Z"}},
					},
				}))
			})
		})

		Context("when a component is paused and has never run", func() {
			BeforeEach(func() {
				healthPayload.Components = []atc.ComponentHealth{
					{
						Name:   atc.ComponentScheduler,
						Status: atc.HealthStatusHealthy,
						Paused: true,
					},
				}
			})

			It("shows only paused in the detail column", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Out).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "subsystem", Color: color.New(color.Bold)},
						{Contents: "status", Color: color.New(color.Bold)},
						{Contents: "detail", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "overall"}, {Contents: "ok", Color: ui.SucceededColor}, {Contents: "2024-01-01T12:00:00Z"}},
						{{Contents: "database"}, {Contents: "healthy", Color: ui.SucceededColor}, {Contents: ""}},
						{{Contents: "workers"}, {Contents: "healthy", Color: ui.SucceededColor}, {Contents: "2/2 running"}},
						{{Contents: "scheduler"}, {Contents: "healthy", Color: ui.SucceededColor}, {Contents: "paused"}},
					},
				}))
			})
		})

		Context("when a component has never run", func() {
			BeforeEach(func() {
				healthPayload.Components = []atc.ComponentHealth{
					{
						Name:   atc.ComponentScheduler,
						Status: atc.HealthStatusHealthy,
					},
				}
			})

			It("shows an empty detail column", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Out).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "subsystem", Color: color.New(color.Bold)},
						{Contents: "status", Color: color.New(color.Bold)},
						{Contents: "detail", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "overall"}, {Contents: "ok", Color: ui.SucceededColor}, {Contents: "2024-01-01T12:00:00Z"}},
						{{Contents: "database"}, {Contents: "healthy", Color: ui.SucceededColor}, {Contents: ""}},
						{{Contents: "workers"}, {Contents: "healthy", Color: ui.SucceededColor}, {Contents: "2/2 running"}},
						{{Contents: "scheduler"}, {Contents: "healthy", Color: ui.SucceededColor}, {Contents: ""}},
					},
				}))
			})
		})
	})

	Context("when the cluster is failing (503)", func() {
		BeforeEach(func() {
			healthPayload.Status = atc.HealthStatusFailing
			healthPayload.Database = atc.DatabaseHealth{
				Status: atc.HealthStatusUnhealthy,
				Error:  "database unreachable",
			}
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/health"),
					ghttp.RespondWithJSONEncoded(503, healthPayload),
				),
			)
		})

		It("prints the table showing failing status and exits 0", func() {
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(gbytes.Say("failing"))
			Expect(sess.Out).To(gbytes.Say("database unreachable"))
		})
	})

	Context("when the server returns an unexpected error", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/health"),
					ghttp.RespondWith(500, ""),
				),
			)
		})

		It("writes an error message to stderr and exits 1", func() {
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))
			Eventually(sess.Err).Should(gbytes.Say("Unexpected Response"))
		})
	})
})
