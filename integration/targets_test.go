package integration_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

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

		if runtime.GOOS == "windows" {
			os.Setenv("USERPROFILE", tmpDir)
		} else {
			os.Setenv("HOME", tmpDir)
		}

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
			})
		})

		Context("when no targets are available", func() {
		})
	})
})
