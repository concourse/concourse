package driver_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/concourse/concourse/worker/baggageclaim/volume"
	"github.com/concourse/concourse/worker/baggageclaim/volume/driver"

	. "github.com/onsi/ginkgo/v2"
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
			rootFile := filepath.Join(rootVolInit.DataPath(), "updated-file")
			err = ioutil.WriteFile(rootFile, []byte("depth-0"), 0644)
			Expect(err).ToNot(HaveOccurred())

			for depth := 1; depth <= 10; depth++ {
				doomedFile := filepath.Join(rootVolInit.DataPath(), fmt.Sprintf("doomed-file-%d", depth))
				err := ioutil.WriteFile(doomedFile, []byte(fmt.Sprintf("i will be removed at depth %d", depth)), 0644)
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

				content, err := ioutil.ReadFile(updateFilePath)
				Expect(string(content)).To(Equal(fmt.Sprintf("depth-%d", depth-1)))

				err = ioutil.WriteFile(updateFilePath, []byte(fmt.Sprintf("depth-%d", depth)), 0644)
				Expect(err).ToNot(HaveOccurred())

				nest = childLive
			}
		})
	})
})
