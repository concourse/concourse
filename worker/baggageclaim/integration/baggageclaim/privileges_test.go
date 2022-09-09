//go:build linux
// +build linux

package integration_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/worker/baggageclaim"
	"github.com/concourse/concourse/worker/baggageclaim/uidgid"
)

var _ = Describe("Privileges", func() {
	var (
		runner *BaggageClaimRunner
		client baggageclaim.Client

		baseVolume   baggageclaim.Volume
		dataFilename string
		linkSentinel string

		sentinelMode os.FileMode
	)

	var maxUID, maxGID int
	mode := 0755 | os.ModeSetuid | os.ModeSetgid
	sentinelMode = 0000

	writeData := func(volumePath string) string {
		filename := randSeq(10)
		newFilePath := filepath.Join(volumePath, filename)

		err := ioutil.WriteFile(newFilePath, []byte(filename), mode)
		Expect(err).NotTo(HaveOccurred())

		return filename
	}

	makeSentinel := func(volumePath string) string {
		sentinel, err := ioutil.TempFile("",
			fmt.Sprintf("baggageclaim_link_sentinel_%d", GinkgoParallelNode()))

		Expect(err).NotTo(HaveOccurred())

		err = os.Chmod(sentinel.Name(), sentinelMode)
		Expect(err).NotTo(HaveOccurred())

		linkName := randSeq(10)
		err = os.Symlink(sentinel.Name(), filepath.Join(volumePath, linkName))
		Expect(err).NotTo(HaveOccurred())
		return sentinel.Name()
	}

	checkMode := func(filePath string, mode os.FileMode) {
		stat, err := os.Stat(filePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(stat.Mode().String()).To(Equal(mode.String()))
	}

	BeforeEach(func() {
		user, err := user.Current()
		Expect(err).NotTo(HaveOccurred())

		if user.Uid != "0" {
			Skip("must be run as root")
			return
		}

		maxUID = uidgid.MustGetMaxValidUID()
		maxGID = uidgid.MustGetMaxValidGID()

		runner = NewRunner(baggageClaimPath, "naive")
		runner.Start()

		client = runner.Client()

		baseVolume, err = client.CreateVolume(ctx, "some-handle", baggageclaim.VolumeSpec{})
		Expect(err).NotTo(HaveOccurred())

		dataFilename = writeData(baseVolume.Path())
		checkMode(filepath.Join(baseVolume.Path(), dataFilename), mode)

		linkSentinel = makeSentinel(baseVolume.Path())
		checkMode(linkSentinel, sentinelMode)
	})

	AfterEach(func() {
		err := os.RemoveAll(linkSentinel)
		Expect(err).NotTo(HaveOccurred())
		runner.Stop()
		runner.Cleanup()
	})

	Describe("creating an unprivileged copy", func() {
		var childVolume baggageclaim.Volume

		BeforeEach(func() {
			var err error
			childVolume, err = client.CreateVolume(ctx, "another-handle", baggageclaim.VolumeSpec{
				Strategy: baggageclaim.COWStrategy{
					Parent: baseVolume,
				},
				Privileged: false,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("maps uid 0 to (MAX_UID)", func() {
			stat, err := os.Stat(filepath.Join(childVolume.Path(), dataFilename))
			Expect(err).ToNot(HaveOccurred())

			sysStat := stat.Sys().(*syscall.Stat_t)
			Expect(sysStat.Uid).To(Equal(uint32(maxUID)))
			Expect(sysStat.Gid).To(Equal(uint32(maxGID)))
			Expect(stat.Mode().String()).To(Equal(mode.String()))
		})

		It("does not affect the host filesystem by following symlinks", func() {
			checkMode(linkSentinel, sentinelMode)
		})

		Describe("streaming out of the volume", func() {
			var tgzStream io.ReadCloser

			BeforeEach(func() {
				var err error
				tgzStream, err = childVolume.StreamOut(context.TODO(), dataFilename, baggageclaim.GzipEncoding)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				tgzStream.Close()
			})

			It("maps uid 0 to uid 0", func() {
				tarStream, err := gzip.NewReader(tgzStream)
				Expect(err).ToNot(HaveOccurred())

				tarReader := tar.NewReader(tarStream)

				header, err := tarReader.Next()
				Expect(err).ToNot(HaveOccurred())

				Expect(header.Name).To(Equal(dataFilename))
				Expect(header.Uid).To(Equal(0))
				Expect(header.Gid).To(Equal(0))
			})

			Describe("streaming in to a privileged volume", func() {
				var privilegedVolume baggageclaim.Volume

				BeforeEach(func() {
					var err error
					privilegedVolume, err = client.CreateVolume(ctx, "privileged-handle", baggageclaim.VolumeSpec{
						Strategy:   baggageclaim.EmptyStrategy{},
						Privileged: true,
					})
					Expect(err).NotTo(HaveOccurred())
				})

				It("maps uid 0 to uid 0", func() {
					err := privilegedVolume.StreamIn(context.TODO(), ".", baggageclaim.GzipEncoding, tgzStream)
					Expect(err).ToNot(HaveOccurred())

					stat, err := os.Stat(filepath.Join(privilegedVolume.Path(), dataFilename))
					Expect(err).ToNot(HaveOccurred())

					sysStat := stat.Sys().(*syscall.Stat_t)
					Expect(sysStat.Uid).To(Equal(uint32(0)))
					Expect(sysStat.Gid).To(Equal(uint32(0)))
					Expect(stat.Mode().String()).To(Equal(mode.String()))
				})
			})
		})

		Describe("converting the volume to privileged", func() {
			BeforeEach(func() {
				Expect(childVolume.SetPrivileged(ctx, true)).To(Succeed())
			})

			It("re-maps (MAX_UID) to uid 0", func() {
				stat, err := os.Stat(filepath.Join(childVolume.Path(), dataFilename))
				Expect(err).ToNot(HaveOccurred())

				sysStat := stat.Sys().(*syscall.Stat_t)
				Expect(sysStat.Uid).To(Equal(uint32(0)))
				Expect(sysStat.Gid).To(Equal(uint32(0)))
				Expect(stat.Mode().String()).To(Equal(mode.String()))
			})

			Describe("streaming out of the volume", func() {
				It("re-maps uid 0 to uid 0", func() {
					tgzStream, err := childVolume.StreamOut(context.TODO(), dataFilename, baggageclaim.GzipEncoding)
					Expect(err).ToNot(HaveOccurred())

					tarStream, err := gzip.NewReader(tgzStream)
					Expect(err).ToNot(HaveOccurred())

					defer tarStream.Close()

					tarReader := tar.NewReader(tarStream)

					header, err := tarReader.Next()
					Expect(err).ToNot(HaveOccurred())

					Expect(header.Name).To(Equal(dataFilename))
					Expect(header.Uid).To(Equal(0))
					Expect(header.Gid).To(Equal(0))
				})
			})
		})

		Context("when making a privileged copy of an unprivileged volume", func() {
			var subChildVolume baggageclaim.Volume

			BeforeEach(func() {
				var err error
				subChildVolume, err = client.CreateVolume(ctx, "yet-another-handle", baggageclaim.VolumeSpec{
					Strategy: baggageclaim.COWStrategy{
						Parent: childVolume,
					},
					Privileged: true,
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("maps (MAX_UID) to 0", func() {
				stat, err := os.Stat(filepath.Join(subChildVolume.Path(), dataFilename))
				Expect(err).ToNot(HaveOccurred())

				sysStat := stat.Sys().(*syscall.Stat_t)
				Expect(sysStat.Uid).To(Equal(uint32(0)))
				Expect(sysStat.Gid).To(Equal(uint32(0)))
				Expect(stat.Mode().String()).To(Equal(mode.String()))
			})

			Describe("converting the volume to unprivileged", func() {
				BeforeEach(func() {
					Expect(subChildVolume.SetPrivileged(ctx, false)).To(Succeed())
				})

				It("re-maps (MAX_UID) to uid 0", func() {
					stat, err := os.Stat(filepath.Join(childVolume.Path(), dataFilename))
					Expect(err).ToNot(HaveOccurred())

					sysStat := stat.Sys().(*syscall.Stat_t)
					Expect(sysStat.Uid).To(Equal(uint32(maxUID)))
					Expect(sysStat.Gid).To(Equal(uint32(maxGID)))
					Expect(stat.Mode().String()).To(Equal(mode.String()))
				})
			})
		})
	})

	Context("creating a privileged copy", func() {
		var childVolume baggageclaim.Volume

		BeforeEach(func() {
			var err error
			childVolume, err = client.CreateVolume(ctx, "another-handle", baggageclaim.VolumeSpec{
				Strategy: baggageclaim.COWStrategy{
					Parent: baseVolume,
				},
				Privileged: true,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("maps uid 0 to uid 0 (no namespacing)", func() {
			stat, err := os.Stat(filepath.Join(childVolume.Path(), dataFilename))
			Expect(err).ToNot(HaveOccurred())

			sysStat := stat.Sys().(*syscall.Stat_t)
			Expect(sysStat.Uid).To(Equal(uint32(0)))
			Expect(sysStat.Gid).To(Equal(uint32(0)))
			Expect(stat.Mode().String()).To(Equal(mode.String()))
		})

		It("does not affect the host filesystem by following symlinks", func() {
			checkMode(linkSentinel, sentinelMode)
		})

		Describe("streaming out of the volume", func() {
			var tgzStream io.ReadCloser

			BeforeEach(func() {
				var err error
				tgzStream, err = childVolume.StreamOut(ctx, dataFilename, baggageclaim.GzipEncoding)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				tgzStream.Close()
			})

			It("maps uid 0 to uid 0", func() {
				tarStream, err := gzip.NewReader(tgzStream)
				Expect(err).ToNot(HaveOccurred())

				tarReader := tar.NewReader(tarStream)

				header, err := tarReader.Next()
				Expect(err).ToNot(HaveOccurred())

				Expect(header.Name).To(Equal(dataFilename))
				Expect(header.Uid).To(Equal(0))
				Expect(header.Gid).To(Equal(0))
			})

			Describe("streaming in to an unprivileged volume", func() {
				var unprivilegedVolume baggageclaim.Volume

				BeforeEach(func() {
					var err error
					unprivilegedVolume, err = client.CreateVolume(ctx, "unprivileged-handle", baggageclaim.VolumeSpec{
						Strategy:   baggageclaim.EmptyStrategy{},
						Privileged: false,
					})
					Expect(err).NotTo(HaveOccurred())
				})

				It("maps uid 0 to (MAX_UID)", func() {
					err := unprivilegedVolume.StreamIn(ctx, ".", baggageclaim.GzipEncoding, tgzStream)
					Expect(err).ToNot(HaveOccurred())

					stat, err := os.Stat(filepath.Join(unprivilegedVolume.Path(), dataFilename))
					Expect(err).ToNot(HaveOccurred())

					sysStat := stat.Sys().(*syscall.Stat_t)
					Expect(sysStat.Uid).To(Equal(uint32(maxUID)))
					Expect(sysStat.Gid).To(Equal(uint32(maxGID)))
					Expect(stat.Mode().String()).To(Equal(mode.String()))
				})
			})
		})
	})
})
