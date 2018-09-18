package integration_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fly CLI", func() {
	var (
		flyCmd *exec.Cmd
		flyrc  string
		tmpDir string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "fly-test")
		Expect(err).NotTo(HaveOccurred())

		os.Setenv("HOME", tmpDir)
		flyrc = filepath.Join(userHomeDir(), ".flyrc")

		flyFixtureFile, err := os.OpenFile("./fixtures/flyrc.yml", os.O_RDONLY, 0600)
		Expect(err).NotTo(HaveOccurred())

		flyFixtureData, err := ioutil.ReadAll(flyFixtureFile)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(flyrc, flyFixtureData, 0600)
		Expect(err).NotTo(HaveOccurred())

		flyCmd = exec.Command(flyPath, "targets")
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
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
						{{Contents: "another-test"}, {Contents: "https://example.com/another-test"}, {Contents: "test"}, {Contents: "Sat, 19 Mar 2016 01:54:30 UTC"}},
						{{Contents: "no-token"}, {Contents: "https://example.com/no-token"}, {Contents: "main"}, {Contents: "n/a"}},
						{{Contents: "omt"}, {Contents: "https://example.com/omt"}, {Contents: "main"}, {Contents: "Mon, 21 Mar 2016 01:54:30 UTC"}},
						{{Contents: "test"}, {Contents: "https://example.com/test"}, {Contents: "test"}, {Contents: "Fri, 25 Mar 2016 23:29:57 UTC"}},
					},
				}))
			})
		})

		Context("when no targets are available", func() {
			BeforeEach(func() {
				os.RemoveAll(flyrc)
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
