package tarfs_test

import (
	"archive/tar"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/concourse/concourse/go-archive/archivetest"
	"github.com/concourse/concourse/go-archive/tarfs"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Extract", func() {
	var extractionSrc io.Reader
	var extractionDest string

	archiveFiles := archivetest.Archive{
		{
			Name: "./",
			Dir:  true,
		},
		{
			Name:       "./some-file",
			Body:       "some-file-contents",
			AccessTime: time.Unix(12345, 0), // dont expect subsecond precision
			ModTime:    time.Unix(98765, 0), // dont expect subsecond precision
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

		extractionSrc, err = archiveFiles.TarStream()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(extractionDest)
	})

	JustBeforeEach(func() {
		err := tarfs.Extract(extractionSrc, extractionDest)
		Expect(err).NotTo(HaveOccurred())
	})

	extractionTest := func() {
		someFile := filepath.Join(extractionDest, "some-file")

		fileContents, err := os.ReadFile(someFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(fileContents)).To(Equal("some-file-contents"))

		stat, err := os.Stat(someFile)
		Expect(err).NotTo(HaveOccurred())
		// can't really assert access time...
		Expect(stat.ModTime()).To(Equal(time.Unix(98765, 0)))

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

	Context("when 'tar' is on the PATH", func() {
		BeforeEach(func() {
			if runtime.GOOS == "windows" {
				Skip("use go archive library only for windows")
			}
			_, err := exec.LookPath("tar")
			Expect(err).NotTo(HaveOccurred())
		})

		It("extracts the TGZ's files, generating directories, and honoring file permissions and symlinks", extractionTest)
	})

	Context("when 'tar' is not in the PATH", func() {
		var oldPATH string

		BeforeEach(func() {
			oldPATH = os.Getenv("PATH")
			Expect(os.Setenv("PATH", "/dev/null")).To(Succeed())

			_, err := exec.LookPath("tar")
			Expect(err).To(HaveOccurred())
		})

		AfterEach(func() {
			Expect(os.Setenv("PATH", oldPATH)).To(Succeed())
		})

		It("extracts the TGZ's files, generating directories, and honoring file permissions and symlinks", extractionTest)
	})
})

var _ = Describe("ExtractEntry", func() {
	var dest string

	BeforeEach(func() {
		var err error
		dest, err = os.MkdirTemp("", "extract-entry")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(dest)
	})

	It("returns a BreakoutError when a hard link points outside the destination", func() {
		header := &tar.Header{
			Typeflag: tar.TypeLink,
			Name:     "malicious-link",
			Linkname: "../outside-file",
		}

		err := tarfs.ExtractEntry(header, dest, strings.NewReader(""), false)
		Expect(err).To(HaveOccurred())

		var breakoutErr tarfs.BreakoutError
		Expect(err).To(BeAssignableToTypeOf(breakoutErr))

		breakoutErr = err.(tarfs.BreakoutError)
		Expect(breakoutErr.HeaderName).To(Equal("malicious-link"))
		Expect(breakoutErr.LinkName).To(Equal("../outside-file"))
	})

	It("returns a BreakoutError when a symlink points outside the destination", func() {
		header := &tar.Header{
			Typeflag: tar.TypeSymlink,
			Name:     "malicious-link",
			Linkname: "../outside-file",
		}

		err := tarfs.ExtractEntry(header, dest, strings.NewReader(""), false)
		Expect(err).To(HaveOccurred())

		var breakoutErr tarfs.BreakoutError
		Expect(err).To(BeAssignableToTypeOf(breakoutErr))

		breakoutErr = err.(tarfs.BreakoutError)
		Expect(breakoutErr.HeaderName).To(Equal("malicious-link"))
		Expect(breakoutErr.LinkName).To(Equal("../outside-file"))
	})
})
