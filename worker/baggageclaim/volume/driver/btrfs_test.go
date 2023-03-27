package driver_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/concourse/worker/baggageclaim/fs"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
	"github.com/concourse/concourse/worker/baggageclaim/volume/driver"
)

var _ = Describe("BtrFS", func() {
	if runtime.GOOS != "linux" {
		fmt.Println("\x1b[33m*** skipping btrfs tests because non-linux ***\x1b[0m")
		return
	}

	var (
		tempDir    string
		fsDriver   *driver.BtrFSDriver
		filesystem *fs.BtrfsFilesystem
		volumeFs   volume.Filesystem
	)

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "baggageclaim_driver_test")
		Expect(err).NotTo(HaveOccurred())

		logger := lagertest.NewTestLogger("fs")

		imagePath := filepath.Join(tempDir, "image.img")
		volumesDir := filepath.Join(tempDir, "mountpoint")

		filesystem = fs.New(logger, imagePath, volumesDir, "mkfs.btrfs")
		err = filesystem.Create(1 * 1024 * 1024 * 1024)
		Expect(err).NotTo(HaveOccurred())

		fsDriver = driver.NewBtrFSDriver(logger, "btrfs")

		volumeFs, err = volume.NewFilesystem(fsDriver, volumesDir)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		err := filesystem.Delete()
		Expect(err).NotTo(HaveOccurred())

		err = os.RemoveAll(tempDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Lifecycle", func() {
		It("can create and delete a subvolume", func() {
			initVol, err := volumeFs.NewVolume("some-volume")
			Expect(err).NotTo(HaveOccurred())

			Expect(initVol.DataPath()).To(BeADirectory())

			checkSubvolume := exec.Command("btrfs", "subvolume", "show", initVol.DataPath())
			session, err := gexec.Start(checkSubvolume, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-session.Exited
			Expect(session).To(gbytes.Say("some-volume"))
			Expect(session).To(gexec.Exit(0))

			err = initVol.Destroy()
			Expect(err).NotTo(HaveOccurred())

			Expect(initVol.DataPath()).NotTo(BeADirectory())
		})

		It("can delete parent volume when it has subvolumes", func() {
			siblingVol, err := volumeFs.NewVolume("sibling-volume")
			Expect(err).NotTo(HaveOccurred())

			parentVol, err := volumeFs.NewVolume("parent-volume")
			Expect(err).NotTo(HaveOccurred())

			dataPath := parentVol.DataPath()

			create := exec.Command("btrfs", "subvolume", "create", filepath.Join(dataPath, "sub"))
			session, err := gexec.Start(create, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			<-session.Exited
			Expect(session).To(gexec.Exit(0))

			create = exec.Command("btrfs", "subvolume", "create", filepath.Join(dataPath, "sub", "sub"))
			session, err = gexec.Start(create, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			<-session.Exited
			Expect(session).To(gexec.Exit(0))

			err = parentVol.Destroy()
			Expect(err).NotTo(HaveOccurred())

			Expect(parentVol.DataPath()).ToNot(BeADirectory())
			Expect(siblingVol.DataPath()).To(BeADirectory())
		})
	})
})
