package tgzfs_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/go-archive/tgzfs"
)

var _ = Describe("Compress", func() {
	var buffer *bytes.Buffer
	var workDir string
	var paths []string
	var compressErr error

	BeforeEach(func() {
		dir, err := os.MkdirTemp("", "archive-dir")
		Expect(err).NotTo(HaveOccurred())

		err = os.Mkdir(filepath.Join(dir, "outer-dir"), 0755)
		Expect(err).NotTo(HaveOccurred())

		err = os.Mkdir(filepath.Join(dir, "outer-dir", "inner-dir"), 0755)
		Expect(err).NotTo(HaveOccurred())

		innerFile, err := os.Create(filepath.Join(dir, "outer-dir", "inner-dir", "some-file"))
		Expect(err).NotTo(HaveOccurred())

		_, err = innerFile.Write([]byte("sup"))
		Expect(err).NotTo(HaveOccurred())

		err = os.Symlink("some-file", filepath.Join(dir, "outer-dir", "inner-dir", "some-symlink"))
		Expect(err).NotTo(HaveOccurred())

		buffer = new(bytes.Buffer)

		workDir = filepath.Join(dir, "outer-dir")

		paths = []string{}
	})

	JustBeforeEach(func() {
		compressErr = tgzfs.Compress(buffer, workDir, paths...)
	})

	examples := func() {
		Context("with . as a path", func() {
			BeforeEach(func() {
				paths = []string{"."}
			})

			It("archives the directory's contents", func() {
				Expect(compressErr).NotTo(HaveOccurred())

				dest, err := os.MkdirTemp("", "extracted")
				Expect(err).NotTo(HaveOccurred())

				defer os.RemoveAll(dest)

				tgzfs.Extract(buffer, dest)

				Expect(os.ReadFile(filepath.Join(dest, "inner-dir", "some-file"))).To(Equal([]byte("sup")))

				Expect(os.Readlink(filepath.Join(dest, "inner-dir", "some-symlink"))).To(Equal("some-file"))
			})
		})

		Context("with a single file as a path", func() {
			BeforeEach(func() {
				paths = []string{"inner-dir/some-file"}
			})

			It("archives the single file at the root", func() {
				Expect(compressErr).NotTo(HaveOccurred())

				gr, err := gzip.NewReader(buffer)
				Expect(err).NotTo(HaveOccurred())

				reader := tar.NewReader(gr)

				header, err := reader.Next()
				Expect(err).NotTo(HaveOccurred())
				Expect(header.Name).To(Equal("inner-dir/some-file"))
				Expect(header.FileInfo().IsDir()).To(BeFalse())

				contents, err := io.ReadAll(reader)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("sup"))

				_, err = reader.Next()
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when there is no file at the given path", func() {
			BeforeEach(func() {
				paths = []string{"barf"}
			})

			It("returns an error", func() {
				Expect(compressErr).To(HaveOccurred())
			})
		})
	}

	Context("with tar in the PATH", func() {
		BeforeEach(func() {
			if runtime.GOOS == "windows" {
				Skip("use go archive library only for windows")
			}
			_, err := exec.LookPath("tar")
			Expect(err).ToNot(HaveOccurred())
		})

		examples()
	})

	Context("with tar not in the PATH", func() {
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

		examples()
	})
})
