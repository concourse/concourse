package integration_test

import (
	"net/http"
	"os/exec"
	"time"

	"github.com/fatih/color"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("get-components", func() {
	var (
		path string
		err  error
	)

	BeforeEach(func() {
		path, err = atc.Routes.CreatePathForRoute(atc.GetComponents, nil)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when components are returned", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithJSONEncoded(http.StatusOK, []atc.Component{
						{
							Name:     "tracker",
							Interval: 30 * time.Second,
							LastRan:  time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
							Paused:   true,
						},
						{
							Name:     "scheduler",
							Interval: 10 * time.Second,
							LastRan:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
							Paused:   false,
						},
					}),
				),
			)
		})

		It("prints components in a table sorted by name", func() {
			Expect(func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "get-components")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gbytes.Say("scheduler"))
				Eventually(sess).Should(gbytes.Say("tracker"))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(2))
		})

		It("prints components as json with --json flag", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "get-components", "--json")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out.Contents()).To(MatchJSON(`[
		{
          "name": "scheduler",
          "interval": 10000000000,
          "last_ran": "2026-01-01T00:00:00Z",
          "paused": false
        },
        {
          "name": "tracker",
          "interval": 30000000000,
          "last_ran": "2026-01-02T00:00:00Z",
          "paused": true
        }]`))
		})
	})

	Context("when there are no components", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithJSONEncoded(http.StatusOK, []atc.Component{}),
				),
			)
		})

		It("prints an empty table", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "get-components")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(PrintTable(ui.Table{
				Headers: ui.TableRow{
					{Contents: "name", Color: color.New(color.Bold)},
					{Contents: "interval", Color: color.New(color.Bold)},
					{Contents: "last ran", Color: color.New(color.Bold)},
					{Contents: "paused", Color: color.New(color.Bold)},
				},
				Data: []ui.TableRow{},
			}))
		})
	})

	Context("when the user is forbidden", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWith(http.StatusForbidden, nil),
				),
			)
		})

		It("returns an error about needing owner role", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "get-components")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess.Err).Should(gbytes.Say("must be an owner of the 'main' team to interact with components"))
			Eventually(sess).Should(gexec.Exit(1))
		})
	})
})
