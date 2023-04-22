package integration_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/concourse/go-archive/tgzfs"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/worker/baggageclaim"
)

var _ = Describe("Import Strategy", func() {
	var (
		runner *BaggageClaimRunner
		client baggageclaim.Client
	)

	BeforeEach(func() {
		runner = NewRunner(baggageClaimPath, "naive")
		runner.Start()
		client = runner.Client()
	})

	AfterEach(func() {
		runner.Stop()
		runner.Cleanup()
	})

	Describe("API", func() {
		Describe("POST /volumes", func() {
			var (
				dir string

				strategy baggageclaim.ImportStrategy

				volume baggageclaim.Volume
			)

			BeforeEach(func() {
				var err error
				dir, err = ioutil.TempDir("", "host_path")
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(filepath.Join(dir, "file-with-perms"), []byte("file-with-perms-contents"), 0600)
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(filepath.Join(dir, "some-file"), []byte("some-file-contents"), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = os.MkdirAll(filepath.Join(dir, "some-dir"), 0755)
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(filepath.Join(dir, "some-dir", "file-in-dir"), []byte("file-in-dir-contents"), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = os.MkdirAll(filepath.Join(dir, "empty-dir"), 0755)
				Expect(err).NotTo(HaveOccurred())

				err = os.MkdirAll(filepath.Join(dir, "dir-with-perms"), 0700)
				Expect(err).NotTo(HaveOccurred())

				strategy = baggageclaim.ImportStrategy{
					Path: dir,
				}
			})

			AfterEach(func() {
				Expect(os.RemoveAll(dir)).To(Succeed())
			})

			assert := func() {
				It("is in the volume dir", func() {
					Expect(volume.Path()).To(HavePrefix(runner.VolumeDir()))
				})

				It("has the correct contents", func() {
					createdDir := volume.Path()

					Expect(createdDir).To(BeADirectory())

					Expect(filepath.Join(createdDir, "some-file")).To(BeARegularFile())
					Expect(ioutil.ReadFile(filepath.Join(createdDir, "some-file"))).To(Equal([]byte("some-file-contents")))

					Expect(filepath.Join(createdDir, "file-with-perms")).To(BeARegularFile())
					Expect(ioutil.ReadFile(filepath.Join(createdDir, "file-with-perms"))).To(Equal([]byte("file-with-perms-contents")))
					fi, err := os.Lstat(filepath.Join(createdDir, "file-with-perms"))
					Expect(err).NotTo(HaveOccurred())
					expectedFI, err := os.Lstat(filepath.Join(dir, "file-with-perms"))
					Expect(err).NotTo(HaveOccurred())
					Expect(fi.Mode()).To(Equal(expectedFI.Mode()))

					Expect(filepath.Join(createdDir, "some-dir")).To(BeADirectory())

					Expect(filepath.Join(createdDir, "some-dir", "file-in-dir")).To(BeARegularFile())
					Expect(ioutil.ReadFile(filepath.Join(createdDir, "some-dir", "file-in-dir"))).To(Equal([]byte("file-in-dir-contents")))
					fi, err = os.Lstat(filepath.Join(createdDir, "some-dir", "file-in-dir"))
					Expect(err).NotTo(HaveOccurred())
					expectedFI, err = os.Lstat(filepath.Join(dir, "some-dir", "file-in-dir"))
					Expect(err).NotTo(HaveOccurred())
					Expect(fi.Mode()).To(Equal(expectedFI.Mode()))
					Expect(filepath.Join(createdDir, "empty-dir")).To(BeADirectory())

					Expect(filepath.Join(createdDir, "dir-with-perms")).To(BeADirectory())
					fi, err = os.Lstat(filepath.Join(createdDir, "dir-with-perms"))
					Expect(err).NotTo(HaveOccurred())
					expectedFI, err = os.Lstat(filepath.Join(dir, "dir-with-perms"))
					Expect(err).NotTo(HaveOccurred())
					Expect(fi.Mode()).To(Equal(expectedFI.Mode()))
				})
			}

			JustBeforeEach(func() {
				var err error
				volume, err = client.CreateVolume(ctx, "some-handle", baggageclaim.VolumeSpec{
					Strategy: strategy,
				})
				Expect(err).NotTo(HaveOccurred())
			})

			assert()

			Context("when the path is a .tgz", func() {
				var tgz *os.File

				BeforeEach(func() {
					var err error
					tgz, err = ioutil.TempFile("", "host_path_archive")
					Expect(err).ToNot(HaveOccurred())

					err = tgzfs.Compress(tgz, strategy.Path, ".")
					Expect(err).ToNot(HaveOccurred())

					Expect(tgz.Close()).To(Succeed())

					strategy.Path = tgz.Name()
				})

				AfterEach(func() {
					Expect(os.RemoveAll(tgz.Name())).To(Succeed())
				})

				assert()
			})
		})
	})
})
