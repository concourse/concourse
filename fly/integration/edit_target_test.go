package integration_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fly CLI", func() {
	Describe("edit target", func() {
		var (
			flyCmd *exec.Cmd
		)

		Context("when no configuration is specified", func() {
			It("should error out", func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "edit-target")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say("error: no attributes specified to update"))
			})
		})

		Describe("valid configuration", func() {
			var (
				flyrc  string
				tmpDir string
			)

			BeforeEach(func() {
				var err error
				tmpDir, err = ioutil.TempDir("", "fly-test")
				Expect(err).NotTo(HaveOccurred())

				if runtime.GOOS == "windows" {
					os.Setenv("USERPROFILE", tmpDir)
					os.Setenv("HOMEPATH", strings.TrimPrefix(tmpDir, os.Getenv("HOMEDRIVE")))
				} else {
					os.Setenv("HOME", tmpDir)
				}

				flyrc = filepath.Join(userHomeDir(), ".flyrc")

				flyFixtureFile, err := os.OpenFile("./fixtures/flyrc.yml", os.O_RDONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				flyFixtureData, err := ioutil.ReadAll(flyFixtureFile)
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(flyrc, flyFixtureData, 0600)
				Expect(err).NotTo(HaveOccurred())

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
						{{Contents: "another-test"}, {Contents: "https://example.com/another-test"}, {Contents: "test"}, {Contents: "Sat, 19 Mar 2016 01:54:30 UTC"}},
						{{Contents: "no-token"}, {Contents: "https://example.com/no-token"}, {Contents: "main"}, {Contents: "n/a"}},
						{{Contents: "omt"}, {Contents: "https://example.com/omt"}, {Contents: "main"}, {Contents: "Mon, 21 Mar 2016 01:54:30 UTC"}},
						{{Contents: "test"}, {Contents: "https://example.com/test"}, {Contents: "test"}, {Contents: "Fri, 25 Mar 2016 23:29:57 UTC"}},
					},
				}))
			})

			AfterEach(func() {
				os.RemoveAll(tmpDir)
			})

			Context("when url configuration is specified", func() {
				It("should update url field of target", func() {
					flyCmd = exec.Command(flyPath, "-t", "test", "edit-target", "--concourse-url", "new-url")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))

					Expect(sess.Out).To(gbytes.Say(`Updated target: test`))

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
							{{Contents: "another-test"}, {Contents: "https://example.com/another-test"}, {Contents: "test"}, {Contents: "Sat, 19 Mar 2016 01:54:30 UTC"}},
							{{Contents: "no-token"}, {Contents: "https://example.com/no-token"}, {Contents: "main"}, {Contents: "n/a"}},
							{{Contents: "omt"}, {Contents: "https://example.com/omt"}, {Contents: "main"}, {Contents: "Mon, 21 Mar 2016 01:54:30 UTC"}},
							{{Contents: "test"}, {Contents: "new-url"}, {Contents: "test"}, {Contents: "Fri, 25 Mar 2016 23:29:57 UTC"}},
						},
					}))
				})
			})

			Context("when team name configuration is specified", func() {
				It("should update team name of target", func() {
					flyCmd = exec.Command(flyPath, "-t", "omt", "edit-target", "--team-name", "new-team")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))

					Expect(sess.Out).To(gbytes.Say(`Updated target: omt`))

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
							{{Contents: "another-test"}, {Contents: "https://example.com/another-test"}, {Contents: "test"}, {Contents: "Sat, 19 Mar 2016 01:54:30 UTC"}},
							{{Contents: "no-token"}, {Contents: "https://example.com/no-token"}, {Contents: "main"}, {Contents: "n/a"}},
							{{Contents: "omt"}, {Contents: "https://example.com/omt"}, {Contents: "new-team"}, {Contents: "Mon, 21 Mar 2016 01:54:30 UTC"}},
							{{Contents: "test"}, {Contents: "https://example.com/test"}, {Contents: "test"}, {Contents: "Fri, 25 Mar 2016 23:29:57 UTC"}},
						},
					}))
				})
			})

			Context("when target name configuration is specified", func() {
				It("should update the target name", func() {
					flyCmd = exec.Command(flyPath, "-t", "another-test", "edit-target", "--target-name", "and-another-test")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))

					Expect(sess.Out).To(gbytes.Say(`Updated target: another-test`))

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
							{{Contents: "and-another-test"}, {Contents: "https://example.com/another-test"}, {Contents: "test"}, {Contents: "Sat, 19 Mar 2016 01:54:30 UTC"}},
							{{Contents: "no-token"}, {Contents: "https://example.com/no-token"}, {Contents: "main"}, {Contents: "n/a"}},
							{{Contents: "omt"}, {Contents: "https://example.com/omt"}, {Contents: "main"}, {Contents: "Mon, 21 Mar 2016 01:54:30 UTC"}},
							{{Contents: "test"}, {Contents: "https://example.com/test"}, {Contents: "test"}, {Contents: "Fri, 25 Mar 2016 23:29:57 UTC"}},
						},
					}))
				})
			})
		})
	})

})
