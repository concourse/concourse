package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func mountAtPath(path string) string {
	diskImage := filepath.Join(path, "image.img")
	mountPath := filepath.Join(path, "mount")

	command := exec.Command(
		fsMounterPath,
		"--disk-image", diskImage,
		"--mount-path", mountPath,
		"--size-in-megabytes", "1024",
	)

	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(session, "10s").Should(gexec.Exit(0))

	return mountPath
}

func unmountAtPath(path string) {
	diskImage := filepath.Join(path, "image.img")
	mountPath := filepath.Join(path, "mount")

	command := exec.Command(
		fsMounterPath,
		"--disk-image", diskImage,
		"--mount-path", mountPath,
		"--remove",
	)

	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(session, "10s").Should(gexec.Exit(0))

}

var _ = Describe("FS Mounter", func() {
	if runtime.GOOS != "linux" {
		fmt.Println("\x1b[33m*** skipping btrfs tests because non-linux ***\x1b[0m")
		return
	}

	var (
		tempDir   string
		mountPath string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "fs_mounter_test")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(tempDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when starting for the first time", func() {
		BeforeEach(func() {
			mountPath = mountAtPath(tempDir)
		})

		AfterEach(func() {
			unmountAtPath(tempDir)
		})

		It("mounts a btrfs volume", func() {
			command := exec.Command(
				"btrfs",
				"subvolume",
				"show",
				mountPath,
			)

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Context("on subsequent runs", func() {
		BeforeEach(func() {
			mountPath = mountAtPath(tempDir)
		})

		AfterEach(func() {
			unmountAtPath(tempDir)
		})

		It("is idempotent", func() {
			path := filepath.Join(mountPath, "filez")
			err := ioutil.WriteFile(path, []byte("contents"), 0755)
			Expect(err).NotTo(HaveOccurred())

			mountPath = mountAtPath(tempDir)

			contents, err := ioutil.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(contents)).To(Equal("contents"))
		})
	})
})
