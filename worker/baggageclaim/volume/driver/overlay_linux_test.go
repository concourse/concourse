package driver_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/concourse/concourse/worker/baggageclaim/volume"
	"github.com/concourse/concourse/worker/baggageclaim/volume/driver"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Overlay", func() {
	Describe("Driver", func() {
		var tmpdir string
		var fs volume.Filesystem

		BeforeEach(func() {
			var err error
			tmpdir, err = ioutil.TempDir("", "overlay-test")
			Expect(err).ToNot(HaveOccurred())

			overlaysDir := filepath.Join(tmpdir, "overlays")
			overlayDriver := driver.NewOverlayDriver(overlaysDir)

			volumesDir := filepath.Join(tmpdir, "volumes")
			fs, err = volume.NewFilesystem(overlayDriver, volumesDir)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpdir)).To(Succeed())
		})

		It("supports nesting >2 levels deep", func() {
			rootVolInit, err := fs.NewVolume("root-vol")
			Expect(err).ToNot(HaveOccurred())

			// write to file under rootVolData
			rootFile := filepath.Join(rootVolInit.DataPath(), "rootFile")
			err = ioutil.WriteFile(rootFile, []byte("root"), 0644)
			Expect(err).ToNot(HaveOccurred())

			doomedFile := filepath.Join(rootVolInit.DataPath(), "doomedFile")
			err = ioutil.WriteFile(doomedFile, []byte("im doomed"), 0644)
			Expect(err).ToNot(HaveOccurred())

			rootVolLive, err := rootVolInit.Initialize()
			Expect(err).ToNot(HaveOccurred())

			defer func() {
				err := rootVolLive.Destroy()
				Expect(err).ToNot(HaveOccurred())
			}()

			childVolInit, err := rootVolLive.NewSubvolume("child-vol")
			Expect(err).ToNot(HaveOccurred())

			// write to file under rootVolData
			chileFilePath := filepath.Join(childVolInit.DataPath(), "rootFile")
			err = ioutil.WriteFile(chileFilePath, []byte("child"), 0644)
			Expect(err).ToNot(HaveOccurred())

			err = os.Remove(filepath.Join(childVolInit.DataPath(), "doomedFile"))
			Expect(err).ToNot(HaveOccurred())

			childVolLive, err := childVolInit.Initialize()
			Expect(err).ToNot(HaveOccurred())

			defer func() {
				err := childVolLive.Destroy()
				Expect(err).ToNot(HaveOccurred())
			}()

			childVol2Init, err := childVolLive.NewSubvolume("child-vol-2")
			Expect(err).ToNot(HaveOccurred())

			childVol2Live, err := childVol2Init.Initialize()
			Expect(err).ToNot(HaveOccurred())

			defer func() {
				err := childVol2Live.Destroy()
				Expect(err).ToNot(HaveOccurred())
			}()

			childVol3Init, err := childVol2Live.NewSubvolume("child-vol-3")
			Expect(err).ToNot(HaveOccurred())

			childVol3Live, err := childVol3Init.Initialize()
			Expect(err).ToNot(HaveOccurred())

			defer func() {
				err := childVol3Live.Destroy()
				Expect(err).ToNot(HaveOccurred())
			}()

			child3FilePath := filepath.Join(childVol3Live.DataPath(), "rootFile")
			content, err := ioutil.ReadFile(child3FilePath)
			Expect(string(content)).To(Equal("child"))

			_, err = os.Stat(filepath.Join(childVol3Live.DataPath(), "doomedFile"))
			Expect(err).To(HaveOccurred())
		})
	})
})
