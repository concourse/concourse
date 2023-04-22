package integration_test

import (
	"os/exec"

	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fly CLI", func() {
	Describe("delete target", func() {
		Describe("valid configuration", func() {
			BeforeEach(func() {
				createFlyRc(rc.Targets{
					"test1": {
						API:      "https://example.com/test1",
						TeamName: "main",
						Token:    &rc.TargetToken{Type: "Bearer", Value: validAccessToken(date(2020, 1, 1))},
					},
					"test2": {
						API:      "https://example.com/test2",
						TeamName: "main",
						Token:    &rc.TargetToken{Type: "Bearer", Value: validAccessToken(date(2020, 1, 2))},
					},
				})

				flyCmd := exec.Command(flyPath, "targets")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))

				Expect(sess.Out).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "name", Color: color.New(color.Bold)},
						{Contents: "url", Color: color.New(color.Bold)},
						{Contents: "team", Color: color.New(color.Bold)},
						{Contents: "expiry", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "test1"}, {Contents: "https://example.com/test1"}, {Contents: "main"}, {Contents: "Wed, 01 Jan 2020 00:00:00 UTC"}},
						{{Contents: "test2"}, {Contents: "https://example.com/test2"}, {Contents: "main"}, {Contents: "Thu, 02 Jan 2020 00:00:00 UTC"}},
					},
				}))
			})

			Context("when fly target is specified", func() {
				It("should delete target", func() {
					flyCmd := exec.Command(flyPath, "-t", "test1", "delete-target")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))

					Expect(sess.Out).To(gbytes.Say(`deleted target: test1`))

					flyCmd = exec.Command(flyPath, "targets")
					sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))
					Expect(sess.Out).To(PrintTable(ui.Table{
						Headers: ui.TableRow{
							{Contents: "name", Color: color.New(color.Bold)},
							{Contents: "url", Color: color.New(color.Bold)},
							{Contents: "team", Color: color.New(color.Bold)},
							{Contents: "expiry", Color: color.New(color.Bold)},
						},
						Data: []ui.TableRow{
							{{Contents: "test2"}, {Contents: "https://example.com/test2"}, {Contents: "main"}, {Contents: "Thu, 02 Jan 2020 00:00:00 UTC"}},
						},
					}))
				})
			})

			Context("when configuration all", func() {
				It("should delete all targets", func() {
					flyCmd := exec.Command(flyPath, "-t", "test1", "delete-target", "--all")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))

					Expect(sess.Out).To(gbytes.Say(`deleted all targets`))

					flyCmd = exec.Command(flyPath, "targets")
					sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))
					Expect(sess.Out).To(PrintTable(ui.Table{
						Headers: ui.TableRow{
							{Contents: "name", Color: color.New(color.Bold)},
							{Contents: "url", Color: color.New(color.Bold)},
							{Contents: "team", Color: color.New(color.Bold)},
							{Contents: "expiry", Color: color.New(color.Bold)},
						},
						Data: []ui.TableRow{},
					}))
				})
			})
		})
	})
})
