package driver_test

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
	"github.com/concourse/concourse/worker/baggageclaim/volume/driver"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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

			knownHandles := map[string]struct{}{
				"known-vol-1": {},
				"known-vol-2": {},
			}

			err := overlayDrv.RemoveOrphanedResources(knownHandles)
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

			knownHandles := map[string]struct{}{}
			err := overlayDrv.RemoveOrphanedResources(knownHandles)
			Expect(err).ToNot(HaveOccurred())

			Expect(filepath.Join(overlaysDir, "work", "work-only-orphan")).ToNot(BeADirectory())
		})

		It("handles an empty overlays directory", func() {
			knownHandles := map[string]struct{}{}
			err := overlayDrv.RemoveOrphanedResources(knownHandles)
			Expect(err).ToNot(HaveOccurred())
		})
	})

})
