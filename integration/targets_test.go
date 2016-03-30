package integration_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fly CLI", func() {
	var (
		flyCmd *exec.Cmd
		tmpDir string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "fly-test")
		Expect(err).ToNot(HaveOccurred())

		flyrc := filepath.Join(userHomeDir(), ".flyrc")

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
				Expect(flyCmd).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "name", Color: color.New(color.Bold)},
						{Contents: "url", Color: color.New(color.Bold)},
						{Contents: "expiry", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "another-test"}, {Contents: "https://example.com/another-test"}, {Contents: "Fri, 18 Mar 2016 18:54:30 PDT"}},
						{{Contents: "omt"}, {Contents: "https://example.com/omt"}, {Contents: "Sun, 20 Mar 2016 18:54:30 PDT"}},
						{{Contents: "test"}, {Contents: "https://example.com/test"}, {Contents: "Fri, 25 Mar 2016 16:29:57 PDT"}},
					},
				}))

				Expect(flyCmd).To(HaveExited(0))
			})
		})

		XContext("when no targets are available", func() {
			It("asks the users to add targets", func() {
			})
		})
	})
})
