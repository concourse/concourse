package volume_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/concourse/concourse/worker/baggageclaim/volume"
	"github.com/concourse/concourse/worker/baggageclaim/volume/volumefakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Filesystem", func() {
	var (
		driver    volumefakes.FakeDriver
		parentDir string
		fs        Filesystem
		logger    *lagertest.TestLogger
	)

	BeforeEach(func() {
		parentDir = GinkgoT().TempDir()
		logger = lagertest.NewTestLogger("fs")
		driver = volumefakes.FakeDriver{}
		f, err := NewFilesystem(logger, &driver, parentDir)
		Expect(err).NotTo(HaveOccurred())
		fs = f
	})

	It("initializes the init/live/dead directories", func() {
		Expect(filepath.Join(parentDir, "init")).To(BeADirectory())
		Expect(filepath.Join(parentDir, "live")).To(BeADirectory())
		Expect(filepath.Join(parentDir, "dead")).To(BeADirectory())
	})

	Describe("NewVolume", func() {
		It("creates a volume", func() {
			driver.CreateVolumeReturns(nil)

			vol, err := fs.NewVolume("some-volume")
			Expect(err).ToNot(HaveOccurred())
			Expect(vol).ToNot(BeNil())

			Expect(vol.Handle()).To(Equal("some-volume"))
			Expect(vol.DataPath()).To(Equal(filepath.Join(parentDir, "init", "some-volume", "volume")))
			Expect(filepath.Dir(vol.DataPath())).To(BeADirectory())

			Expect(driver.CreateVolumeCallCount()).To(Equal(1))
		})

		It("destroys the volume if there is an error", func() {
			driver.CreateVolumeReturns(errors.New("some-error"))
			driver.DestroyVolumeReturns(nil)

			vol, err := fs.NewVolume("some-volume")
			Expect(err).To(HaveOccurred())
			Expect(vol).To(BeNil())

			Expect(filepath.Join(parentDir, "init", "some-volume")).ToNot(BeADirectory())

			Expect(driver.CreateVolumeCallCount()).To(Equal(1))
			Expect(driver.DestroyVolumeCallCount()).To(Equal(1))
		})

		It("initializing a volume returns a FilesystemLiveVolume", func() {
			driver.CreateVolumeReturns(nil)

			iVol, err := fs.NewVolume("some-volume")
			Expect(err).NotTo(HaveOccurred())
			Expect(iVol).ToNot(BeNil())
			Expect(iVol).ToNot(BeAssignableToTypeOf(volumefakes.FakeFilesystemInitVolume{}))

			lVol, err := iVol.Initialize()
			Expect(err).NotTo(HaveOccurred())
			Expect(lVol).ToNot(BeNil())
			Expect(lVol).ToNot(BeAssignableToTypeOf(volumefakes.FakeFilesystemLiveVolume{}))

			Expect(lVol.Handle()).To(Equal("some-volume"))
			Expect(lVol.DataPath()).To(Equal(filepath.Join(parentDir, "live", "some-volume", "volume")))
			Expect(filepath.Dir(lVol.DataPath())).To(BeADirectory())
		})
	})

	Describe("LookupVolume", func() {
		It("returns a volume if found", func() {
			driver.CreateVolumeReturns(nil)

			iVol, err := fs.NewVolume("some-volume")
			Expect(err).NotTo(HaveOccurred())
			Expect(iVol).ToNot(BeNil())

			lVol, err := iVol.Initialize()
			Expect(err).NotTo(HaveOccurred())
			Expect(lVol).ToNot(BeNil())

			actual, found, err := fs.LookupVolume("some-volume")
			Expect(err).To(BeNil())
			Expect(found).To(BeTrue())
			Expect(actual).ToNot(BeNil())
			Expect(actual).To(BeEquivalentTo(lVol))
		})

		It("returns false if the volume does not exist", func() {
			actual, found, err := fs.LookupVolume("some-volume")
			Expect(err).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(actual).To(BeNil())
		})

		It("returns false if the volume is not a directory", func() {
			f, err := os.Create(filepath.Join(parentDir, "live", "some-volume"))
			Expect(err).To(BeNil())
			defer f.Close()

			actual, found, err := fs.LookupVolume("some-volume")
			Expect(err).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(actual).To(BeNil())
		})
	})

	Describe("ListVolumes", func() {
		It("returns all volumes under the live directory", func() {
			os.Mkdir(filepath.Join(parentDir, "live", "vol1"), 0755)
			os.Mkdir(filepath.Join(parentDir, "live", "vol2"), 0755)
			os.Mkdir(filepath.Join(parentDir, "live", "vol3"), 0755)
			// Should not return volumes in init and dead
			os.Mkdir(filepath.Join(parentDir, "init", "vol4"), 0755)
			os.Mkdir(filepath.Join(parentDir, "dead", "vol5"), 0755)

			vols, err := fs.ListVolumes()
			Expect(err).To(BeNil())
			Expect(len(vols)).To(Equal(3))
			for i, v := range vols {
				p := filepath.Join(parentDir, "live", fmt.Sprintf("vol%d", i+1), "volume")
				Expect(v.DataPath()).To(Equal(p))
			}
		})
	})

	Describe("Volume Lifecycle", func() {
		It("creates sub-volumes and tracks parent volume", func() {
			driver.CreateVolumeReturns(nil)

			ivol, err := fs.NewVolume("parent-vol")
			Expect(err).ToNot(HaveOccurred())
			Expect(ivol).ToNot(BeNil())

			lvol, err := ivol.Initialize()
			Expect(err).ToNot(HaveOccurred())
			Expect(lvol).ToNot(BeNil())

			parent, hasParent, err := lvol.Parent()
			Expect(err).ToNot(HaveOccurred())
			Expect(hasParent).To(BeFalse())
			Expect(parent).To(BeNil(), "parent volume should have no parent")

			sivol, err := lvol.NewSubvolume("sub-vol")
			Expect(err).ToNot(HaveOccurred())
			Expect(sivol).ToNot(BeNil())

			slvol, err := sivol.Initialize()
			Expect(err).ToNot(HaveOccurred())
			Expect(slvol).ToNot(BeNil())
			Expect(slvol.Handle()).To(Equal("sub-vol"))

			parent, hasParent, err = slvol.Parent()
			Expect(err).ToNot(HaveOccurred())
			Expect(hasParent).To(BeTrue())
			Expect(parent.Handle()).To(Equal("parent-vol"))
		})

		It("destroys volume successfully", func() {
			driver.CreateVolumeReturns(nil)

			ivol, err := fs.NewVolume("some-volume")
			Expect(err).ToNot(HaveOccurred())
			Expect(ivol).ToNot(BeNil())

			lvol, err := ivol.Initialize()
			Expect(err).ToNot(HaveOccurred())
			Expect(lvol).ToNot(BeNil())

			driver.DestroyVolumeReturns(nil)
			err = lvol.Destroy()
			Expect(err).ToNot(HaveOccurred())
			Expect(filepath.Join(parentDir, "dead", "some-volume")).ToNot(BeADirectory(), "volume should not exist")
			Expect(filepath.Join(parentDir, "live", "some-volume")).ToNot(BeADirectory(), "volume should not exist")
			Expect(driver.DestroyVolumeCallCount()).To(Equal(1))
		})

		It("cleans up if creating a sub-volume errors", func() {
			driver.CreateVolumeReturns(nil)

			ivol, err := fs.NewVolume("some-volume")
			Expect(err).ToNot(HaveOccurred())
			Expect(ivol).ToNot(BeNil())

			lvol, err := ivol.Initialize()
			Expect(err).ToNot(HaveOccurred())
			Expect(lvol).ToNot(BeNil())

			driver.CreateCopyOnWriteLayerReturns(errors.New("cow-error"))
			driver.DestroyVolumeReturns(errors.New("driver-destroy-error"))
			svol, err := lvol.NewSubvolume("sub-volume")
			Expect(err).To(MatchError("cow-error"))
			Expect(svol).To(BeNil())

			Expect(driver.CreateCopyOnWriteLayerCallCount()).To(Equal(1))
			Expect(driver.DestroyVolumeCallCount()).To(Equal(1))

			logs := logger.Logs()
			Expect(len(logs)).To(Equal(5), "should log the Driver.DestroyVolume() error")
			Expect(logs[4].Message).To(Equal("fs.filesystem.driver-destroy-volume"))
			Expect(logs[4].LogLevel).To(Equal(lager.ERROR))
			Expect(logs[4].Data["error"]).To(Equal("driver-destroy-error"))
			Expect(logs[4].Data["handle"]).To(Equal("sub-volume"))
		})
	})

	Describe("CleanupOrphanedEntries", func() {
		It("destroys stale dead/ volume entries", func() {
			// Create dead volume directories
			Expect(os.Mkdir(filepath.Join(parentDir, "dead/dead-vol-1"), 0755)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(parentDir, "dead/dead-vol-2"), 0755)).To(Succeed())

			err := fs.CleanupOrphanedEntries()
			Expect(err).ToNot(HaveOccurred())

			// Dead dirs should be removed
			Expect(filepath.Join(parentDir, "dead/dead-vol-1")).ToNot(BeADirectory())
			Expect(filepath.Join(parentDir, "dead/dead-vol-2")).ToNot(BeADirectory())

			// DestroyVolume called once per dead volume
			Expect(driver.DestroyVolumeCallCount()).To(Equal(2))
		})

		It("passes a live isKnown checker for init and live handles", func() {
			// Create live and init volume directories
			Expect(os.Mkdir(filepath.Join(parentDir, "live/live-vol-1"), 0755)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(parentDir, "live/live-vol-2"), 0755)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(parentDir, "init/init-vol-1"), 0755)).To(Succeed())

			err := fs.CleanupOrphanedEntries()
			Expect(err).ToNot(HaveOccurred())
			Expect(driver.RemoveOrphanedResourcesCallCount()).To(Equal(1))
			volumeExists := driver.RemoveOrphanedResourcesArgsForCall(0)
			Expect(volumeExists("live-vol-1")).To(BeTrue())
			Expect(volumeExists("live-vol-2")).To(BeTrue())
			Expect(volumeExists("init-vol-1")).To(BeTrue())
			Expect(volumeExists("unknown-vol")).To(BeFalse())
		})

		It("volumeExists checks init/ first, then live/", func() {
			pathKnownCallCount := 0
			f, err := NewFilesystem(logger, &driver, parentDir,
				WithPathKnownFunc(func(path string) bool {
					pathKnownCallCount++
					switch pathKnownCallCount {
					case 1, 2:
						Expect(path).To(Equal(filepath.Join(parentDir, "init/promoting")))
						return true
					case 3:
						Expect(path).To(Equal(filepath.Join(parentDir, "live/promoting")))
						return true
					default:
						Fail("Unknown test situation! Think about it.")
						return false
					}
				}))
			Expect(err).ToNot(HaveOccurred())
			fs = f

			err = fs.CleanupOrphanedEntries()
			Expect(err).ToNot(HaveOccurred())
			Expect(driver.RemoveOrphanedResourcesCallCount()).To(Equal(1))
			volumeExists := driver.RemoveOrphanedResourcesArgsForCall(0)

			Expect(os.Mkdir(filepath.Join(parentDir, "init/promoting"), 0755)).To(Succeed())
			Expect(volumeExists("promoting")).To(BeTrue())
			Expect(os.Rename(
				filepath.Join(parentDir, "init/promoting"),
				filepath.Join(parentDir, "live/promoting"),
			)).To(Succeed())
			Expect(volumeExists("promoting")).To(BeTrue())
		})

		It("treats non-ErrNotExist errors as 'known' so volumes are not mass-deleted", func() {
			err := fs.CleanupOrphanedEntries()
			Expect(err).ToNot(HaveOccurred())

			isKnown := driver.RemoveOrphanedResourcesArgsForCall(0)
			Expect(isKnown("unknown-vol")).To(BeFalse())

			initDir := filepath.Join(parentDir, "init")
			liveDir := filepath.Join(parentDir, "live")
			By("removing the init/ and live/ dirs and replacing them with files. This will cause os.Stat to return ENOTDIR")
			Expect(os.RemoveAll(initDir)).To(Succeed())
			Expect(os.RemoveAll(liveDir)).To(Succeed())
			Expect(os.WriteFile(initDir, nil, 0644)).To(Succeed())
			Expect(os.WriteFile(liveDir, nil, 0644)).To(Succeed())

			Expect(isKnown("unknown-vol")).To(BeTrue(),
				"A Stat error other than ErrNotExist must NOT be treated as 'unknown'")
		})

		It("logs errors from dead volume cleanup but continues", func() {
			driver.DestroyVolumeReturns(errors.New("destroy-failed"))

			Expect(os.Mkdir(filepath.Join(parentDir, "dead/dead-vol"), 0755)).To(Succeed())

			err := fs.CleanupOrphanedEntries()
			Expect(err).ToNot(HaveOccurred())

			// Dead dir should still be cleaned up (os.RemoveAll after driver call)
			Expect(filepath.Join(parentDir, "dead/dead-vol")).ToNot(BeADirectory())
		})

		It("works with empty directories", func() {
			err := fs.CleanupOrphanedEntries()
			Expect(err).ToNot(HaveOccurred())

			Expect(driver.RemoveOrphanedResourcesCallCount()).To(Equal(1))
			isKnown := driver.RemoveOrphanedResourcesArgsForCall(0)
			Expect(isKnown("anything")).To(BeFalse())
		})
	})
})
