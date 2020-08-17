package integration_test

import (
	"os/exec"

	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("logout Command", func() {

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

	Describe("missing parameters", func() {
		Context("when validating parameters", func() {
			It("instructs the user to specify --target or --all if both are missing", func() {
				flyCmd := exec.Command(flyPath, "logout")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say(`must specify either --target or --all`))
			})

			It("instructs the user to specify --target or --all if both are present", func() {
				flyCmd := exec.Command(flyPath, "logout", "--target", "some-target", "--all")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say(`must specify either --target or --all`))
			})
		})
	})

	Describe("delete all", func() {
		It("removes all tokens and all targets remain in flyrc", func() {
			flyCmd := exec.Command(flyPath, "logout", "--all")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))

			Expect(sess.Out).To(gbytes.Say(`logged out of all targets`))

			flyCmd = exec.Command(flyPath, "targets")
			sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(PrintTable(ui.Table{
				Headers: ui.TableRow{
					{Contents: "name", Color: color.New(color.Bold)},
					{Contents: "url", Color: color.New(color.Bold)},
					{Contents: "expiry", Color: color.New(color.Bold)},
				},
				Data: []ui.TableRow{
					{{Contents: "test1"}, {Contents: "https://example.com/test1"}, {Contents: "main"}, {Contents: "n/a"}},
					{{Contents: "test2"}, {Contents: "https://example.com/test2"}, {Contents: "main"}, {Contents: "n/a"}},
				},
			}))
		})
	})

	Describe("delete one", func() {
		It("removes token of the target and the target should remain in .flyrc", func() {
			flyCmd := exec.Command(flyPath, "logout", "-t", "test2")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))

			Expect(sess.Out).To(gbytes.Say(`logged out of target: test2`))

			flyCmd = exec.Command(flyPath, "targets")
			sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(PrintTable(ui.Table{
				Headers: ui.TableRow{
					{Contents: "name", Color: color.New(color.Bold)},
					{Contents: "url", Color: color.New(color.Bold)},
					{Contents: "expiry", Color: color.New(color.Bold)},
				},
				Data: []ui.TableRow{
					{{Contents: "test1"}, {Contents: "https://example.com/test1"}, {Contents: "main"}, {Contents: "Wed, 01 Jan 2020 00:00:00 UTC"}},
					{{Contents: "test2"}, {Contents: "https://example.com/test2"}, {Contents: "main"}, {Contents: "n/a"}},
				},
			}))
		})
	})
})
