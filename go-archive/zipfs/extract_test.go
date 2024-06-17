package zipfs_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/concourse/concourse/go-archive/archivetest"
	"github.com/concourse/concourse/go-archive/zipfs"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Extract", func() {
	var extractionSrc string
	var extractionDest string

	archiveFiles := archivetest.Archive{
		{
			Name: "./",
			Dir:  true,
		},
		{
			Name: "./some-file",
			Body: "some-file-contents",
		},
		{
			Name: "./empty-dir/",
			Dir:  true,
		},
		{
			Name: "./nonempty-dir/",
			Dir:  true,
		},
		{
			Name: "./nonempty-dir/file-in-dir",
			Body: "file-in-dir-contents",
		},
		{
			Name: "./legit-exe-not-a-virus.bat",
			Mode: 0644,
			Body: "rm -rf /",
		},
		{
			Name: "./some-symlink",
			Link: "some-file",
			Mode: 0755,
		},
	}

	BeforeEach(func() {
		var err error

		extractionDest, err = os.MkdirTemp("", "extracted")
		Expect(err).NotTo(HaveOccurred())

		extractionSrc, err = archiveFiles.ZipFile("")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(extractionSrc)
		os.RemoveAll(extractionDest)
	})

	JustBeforeEach(func() {
		err := zipfs.Extract(extractionSrc, extractionDest)
		Expect(err).NotTo(HaveOccurred())
	})

	extractionTest := func() {
		fileContents, err := os.ReadFile(filepath.Join(extractionDest, "some-file"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(fileContents)).To(Equal("some-file-contents"))

		fileContents, err = os.ReadFile(filepath.Join(extractionDest, "nonempty-dir", "file-in-dir"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(fileContents)).To(Equal("file-in-dir-contents"))

		executable, err := os.Open(filepath.Join(extractionDest, "legit-exe-not-a-virus.bat"))
		Expect(err).NotTo(HaveOccurred())

		executableInfo, err := executable.Stat()
		Expect(err).NotTo(HaveOccurred())
		if runtime.GOOS != "windows" {
			Expect(executableInfo.Mode()).To(Equal(os.FileMode(0644)))
		}

		emptyDir, err := os.Open(filepath.Join(extractionDest, "empty-dir"))
		Expect(err).NotTo(HaveOccurred())

		emptyDirInfo, err := emptyDir.Stat()
		Expect(err).NotTo(HaveOccurred())

		Expect(emptyDirInfo.IsDir()).To(BeTrue())

		target, err := os.Readlink(filepath.Join(extractionDest, "some-symlink"))
		Expect(err).NotTo(HaveOccurred())
		Expect(target).To(Equal("some-file"))

		symlinkInfo, err := os.Lstat(filepath.Join(extractionDest, "some-symlink"))
		Expect(err).NotTo(HaveOccurred())

		if runtime.GOOS != "windows" {
			Expect(symlinkInfo.Mode() & 0755).To(Equal(os.FileMode(0755)))
		}
	}

	Context("when 'unzip' is on the PATH", func() {
		BeforeEach(func() {
			if runtime.GOOS == "windows" {
				Skip("unzip is not valid on Windows")
			}
			_, err := exec.LookPath("unzip")
			Expect(err).NotTo(HaveOccurred())
		})

		It("extracts the ZIP's files, generating directories, and honoring file permissions and symlinks", extractionTest)
	})

	Context("when 'unzip' is not in the PATH", func() {
		var oldPATH string

		BeforeEach(func() {
			oldPATH = os.Getenv("PATH")
			os.Setenv("PATH", "/dev/null")

			_, err := exec.LookPath("unzip")
			Expect(err).To(HaveOccurred())
		})

		AfterEach(func() {
			os.Setenv("PATH", oldPATH)
		})

		It("extracts the ZIP's files, generating directories, and honoring file permissions and symlinks", extractionTest)
	})
})
