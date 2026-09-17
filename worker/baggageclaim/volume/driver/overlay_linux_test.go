package driver_test

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
	"github.com/concourse/concourse/worker/baggageclaim/volume/driver"
	"github.com/concourse/concourse/worker/baggageclaim/volume/volumefakes"
	"github.com/moby/sys/mountinfo"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func knownHandles(handles ...string) func(string) bool {
	set := make(map[string]struct{}, len(handles))
	for _, h := range handles {
		set[h] = struct{}{}
	}
	return func(handle string) bool {
		_, ok := set[handle]
		return ok
	}
}

func countMountsUnder(path string) int {
	mounts, _ := mountinfo.GetMounts(mountinfo.PrefixFilter(path))
	return len(mounts)
}

var _ = Describe("Overlay", func() {
	Context("Driver", func() {
		var tmpdir string
		var fs volume.Filesystem

		BeforeEach(func() {
			var err error
			tmpdir, err = os.MkdirTemp("", "overlay-test")
			Expect(err).ToNot(HaveOccurred())

			logger := lagertest.NewTestLogger("fs")

			overlaysDir := filepath.Join(tmpdir, "overlays")
			overlayDriver := driver.NewOverlayDriver(logger, overlaysDir)

			volumesDir := filepath.Join(tmpdir, "volumes")
			fs, err = volume.NewFilesystem(logger, overlayDriver, volumesDir)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpdir)).To(Succeed())
		})

		It("supports nesting >2 levels deep", func() {
			rootVolInit, err := fs.NewVolume("root-vol")
			Expect(err).ToNot(HaveOccurred())

			// write to file under rootVolData
			rootFile := filepath.Join(rootVolInit.DataPath(), "updated-file")
			err = os.WriteFile(rootFile, []byte("depth-0"), 0644)
			Expect(err).ToNot(HaveOccurred())

			for depth := 1; depth <= 10; depth++ {
				doomedFile := filepath.Join(rootVolInit.DataPath(), fmt.Sprintf("doomed-file-%d", depth))
				err := os.WriteFile(doomedFile, fmt.Appendf([]byte{}, "i will be removed at depth %d", depth), 0644)
				Expect(err).ToNot(HaveOccurred())
			}

			rootVolLive, err := rootVolInit.Initialize()
			Expect(err).ToNot(HaveOccurred())

			defer func() {
				err := rootVolLive.Destroy()
				Expect(err).ToNot(HaveOccurred())
			}()

			nest := rootVolLive
			for depth := 1; depth <= 10; depth++ {
				By(fmt.Sprintf("creating a child nested %d levels deep", depth))

				childInit, err := nest.NewSubvolume(fmt.Sprintf("child-vol-%d", depth))
				Expect(err).ToNot(HaveOccurred())

				childLive, err := childInit.Initialize()
				Expect(err).ToNot(HaveOccurred())

				defer func() {
					err := childLive.Destroy()
					Expect(err).ToNot(HaveOccurred())
				}()

				for i := 1; i <= 10; i++ {
					doomedFilePath := filepath.Join(childLive.DataPath(), fmt.Sprintf("doomed-file-%d", i))

					_, statErr := os.Stat(doomedFilePath)
					if i < depth {
						Expect(statErr).To(HaveOccurred())
					} else {
						Expect(statErr).ToNot(HaveOccurred())

						if i == depth {
							err := os.Remove(doomedFilePath)
							Expect(err).ToNot(HaveOccurred())
						}
					}
				}

				updateFilePath := filepath.Join(childLive.DataPath(), "updated-file")

				content, err := os.ReadFile(updateFilePath)
				Expect(string(content)).To(Equal(fmt.Sprintf("depth-%d", depth-1)))

				err = os.WriteFile(updateFilePath, fmt.Appendf([]byte{}, "depth-%d", depth), 0644)
				Expect(err).ToNot(HaveOccurred())

				nest = childLive
			}
		})
	})

	Context("RemoveOrphanedResources", func() {
		var (
			overlaysDir string
			overlayDrv  volume.Driver
		)

		BeforeEach(func() {
			tmpdir, err := os.MkdirTemp("", "overlay-orphan-test")
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { os.RemoveAll(tmpdir) })

			overlaysDir = filepath.Join(tmpdir, "overlays")
			Expect(os.MkdirAll(filepath.Join(overlaysDir, "work"), 0755)).To(Succeed())

			logger := lagertest.NewTestLogger("overlay-orphan")
			overlayDrv = driver.NewOverlayDriver(logger, overlaysDir)
		})

		It("removes orphaned layer and work dirs while preserving known handles", func() {
			// Create known handles
			Expect(os.Mkdir(filepath.Join(overlaysDir, "known-vol-1"), 0755)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(overlaysDir, "work", "known-vol-1"), 0755)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(overlaysDir, "known-vol-2"), 0755)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(overlaysDir, "work", "known-vol-2"), 0755)).To(Succeed())

			// Create orphaned handles
			Expect(os.Mkdir(filepath.Join(overlaysDir, "orphan-vol-1"), 0755)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(overlaysDir, "work", "orphan-vol-1"), 0755)).To(Succeed())
			// orphan-vol-2 has no corresponding work dir
			Expect(os.Mkdir(filepath.Join(overlaysDir, "orphan-vol-2"), 0755)).To(Succeed())

			err := overlayDrv.RemoveOrphanedResources(knownHandles("known-vol-1", "known-vol-2"))
			Expect(err).ToNot(HaveOccurred())

			// Known handles should still exist
			Expect(filepath.Join(overlaysDir, "known-vol-1")).To(BeADirectory())
			Expect(filepath.Join(overlaysDir, "work", "known-vol-1")).To(BeADirectory())
			Expect(filepath.Join(overlaysDir, "known-vol-2")).To(BeADirectory())
			Expect(filepath.Join(overlaysDir, "work", "known-vol-2")).To(BeADirectory())

			// Orphaned layer dirs should be removed
			Expect(filepath.Join(overlaysDir, "orphan-vol-1")).ToNot(BeADirectory())
			Expect(filepath.Join(overlaysDir, "orphan-vol-2")).ToNot(BeADirectory())

			// Orphaned work dirs should be removed
			Expect(filepath.Join(overlaysDir, "work", "orphan-vol-1")).ToNot(BeADirectory())
		})

		It("removes orphaned work-only dirs (no corresponding layer dir)", func() {
			// Create a work dir with no layer dir
			Expect(os.Mkdir(filepath.Join(overlaysDir, "work", "work-only-orphan"), 0755)).To(Succeed())

			err := overlayDrv.RemoveOrphanedResources(knownHandles())
			Expect(err).ToNot(HaveOccurred())

			Expect(filepath.Join(overlaysDir, "work", "work-only-orphan")).ToNot(BeADirectory())
		})

		It("errors if nil is given instead of a func", func() {
			err := overlayDrv.RemoveOrphanedResources(nil)
			Expect(err).To(MatchError("must pass in a function"))
		})

		It("handles an empty overlays directory", func() {
			err := overlayDrv.RemoveOrphanedResources(knownHandles())
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the overlays dir does not exist", func() {
			BeforeEach(func() {
				err := os.RemoveAll(overlaysDir)
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not error", func() {
				err := overlayDrv.RemoveOrphanedResources(knownHandles())
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Context("Recover", func() {
		var (
			tmpdir     string
			volumesDir string
			drv        volume.Driver
			realFS     volume.Filesystem
		)

		BeforeEach(func() {
			var err error
			tmpdir, err = os.MkdirTemp("", "overlay-recover-test")
			Expect(err).ToNot(HaveOccurred())

			logger := lagertest.NewTestLogger("recover")
			overlaysDir := filepath.Join(tmpdir, "overlays")
			volumesDir = filepath.Join(tmpdir, "volumes")

			drv = driver.NewOverlayDriver(logger, overlaysDir)
			realFS, err = volume.NewFilesystem(logger, drv, volumesDir)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			mounts, _ := mountinfo.GetMounts(mountinfo.PrefixFilter(tmpdir))
			for _, m := range mounts {
				_ = syscall.Unmount(m.Mountpoint, syscall.MNT_DETACH)
			}
			Expect(os.RemoveAll(tmpdir)).To(Succeed())
		})

		It("is idempotent: repeated Recover() calls do not stack mounts", func() {
			vol1init, err := realFS.NewVolume("vol-1")
			Expect(err).ToNot(HaveOccurred())
			vol1, err := vol1init.Initialize()
			Expect(err).ToNot(HaveOccurred())

			vol2init, err := realFS.NewVolume("vol-2")
			Expect(err).ToNot(HaveOccurred())
			vol2, err := vol2init.Initialize()
			Expect(err).ToNot(HaveOccurred())

			baseline := countMountsUnder(volumesDir)
			Expect(baseline).To(Equal(2))

			for i := range 3 {
				Expect(drv.Recover(realFS)).To(Succeed())
				Expect(countMountsUnder(volumesDir)).To(Equal(baseline),
					"Recover() pass %d stacked mounts", i+1)
			}

			Expect(vol1.Destroy()).To(Succeed())
			Expect(vol2.Destroy()).To(Succeed())
		})

		It("rolls back mounts from this pass when a later volume fails to mount", func() {
			vol1init, err := realFS.NewVolume("vol-1")
			Expect(err).ToNot(HaveOccurred())
			vol1, err := vol1init.Initialize()
			Expect(err).ToNot(HaveOccurred())
			defer func() { _ = vol1.Destroy() }()

			vol2init, err := realFS.NewVolume("vol-2")
			Expect(err).ToNot(HaveOccurred())
			vol2, err := vol2init.Initialize()
			Expect(err).ToNot(HaveOccurred())
			defer func() { _ = vol2.Destroy() }()

			// Unmount both volumes to simulate a clean-restart state.
			Expect(syscall.Unmount(vol1.DataPath(), 0)).To(Succeed())
			Expect(syscall.Unmount(vol2.DataPath(), 0)).To(Succeed())

			// Build a fake filesystem: 2 real volumes + 1 whose DataPath does not exist.
			badVol := &volumefakes.FakeFilesystemLiveVolume{}
			badVol.HandleReturns("bad-vol")
			badVol.DataPathReturns("/nonexistent/bad-vol/volume")
			badVol.ParentReturns(nil, false, nil)

			fakeFS := &volumefakes.FakeFilesystem{}
			fakeFS.ListVolumesReturns([]volume.FilesystemLiveVolume{vol1, vol2, badVol}, nil)

			err = drv.Recover(fakeFS)
			Expect(err).To(HaveOccurred())

			mounted1, err := mountinfo.Mounted(vol1.DataPath())
			Expect(err).ToNot(HaveOccurred())
			Expect(mounted1).To(BeFalse(), "vol-1 should have been rolled back")

			mounted2, err := mountinfo.Mounted(vol2.DataPath())
			Expect(err).ToNot(HaveOccurred())
			Expect(mounted2).To(BeFalse(), "vol-2 should have been rolled back")
		})
	})

})
