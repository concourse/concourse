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
	var (
		flyCmd  *exec.Cmd
		targets rc.Targets
	)

	JustBeforeEach(func() {
		createFlyRc(targets)

		flyCmd = exec.Command(flyPath, "targets")
	})

	BeforeEach(func() {
		targets = rc.Targets{
			"another-test": {
				API:      "https://example.com/another-test",
				TeamName: "test",
				Token:    &rc.TargetToken{Type: "Bearer", Value: validAccessToken(date(2020, 1, 1))},
			},
			"no-token": {
				API:      "https://example.com/no-token",
				TeamName: "main",
				Token:    nil,
			},
			"omt": {
				API:      "https://example.com/omt",
				TeamName: "main",
				Token:    &rc.TargetToken{Type: "Bearer", Value: validAccessToken(date(2020, 1, 2))},
			},
			"test": {
				API:      "https://example.com/test",
				TeamName: "test",
				Token:    &rc.TargetToken{Type: "Bearer", Value: validAccessToken(date(2020, 1, 3))},
			},
		}
	})

	Describe("targets", func() {
		Context("when there are targets in the .flyrc", func() {
			It("displays all the targets with their token expiration", func() {
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
						{{Contents: "another-test"}, {Contents: "https://example.com/another-test"}, {Contents: "test"}, {Contents: "Wed, 01 Jan 2020 00:00:00 UTC"}},
						{{Contents: "no-token"}, {Contents: "https://example.com/no-token"}, {Contents: "main"}, {Contents: "n/a"}},
						{{Contents: "omt"}, {Contents: "https://example.com/omt"}, {Contents: "main"}, {Contents: "Thu, 02 Jan 2020 00:00:00 UTC"}},
						{{Contents: "test"}, {Contents: "https://example.com/test"}, {Contents: "test"}, {Contents: "Fri, 03 Jan 2020 00:00:00 UTC"}},
					},
				}))
			})
		})

		Context("when the .flyrc contains a target with an invalid token", func() {
			BeforeEach(func() {
				targets = rc.Targets{
					"test": {
						API:   "https://example.com/test",
						Token: &rc.TargetToken{Type: "Bearer", Value: "banana"},
					},
				}
			})

			It("indicates the token is invalid", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))

				Expect(sess).Should(gbytes.Say("n/a: invalid token"))
			})
		})

		Context("when no targets are available", func() {
			BeforeEach(func() {
				createFlyRc(nil)
			})

			It("prints an empty table", func() {
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
					Data: []ui.TableRow{}}))
			})
		})
	})
})
