package volume_test

import (
	"errors"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/concourse/concourse/worker/baggageclaim/volume"
	"github.com/concourse/concourse/worker/baggageclaim/volume/volumefakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ImportStrategy", func() {
	var (
		driver     volumefakes.FakeDriver
		parentDir  string
		importPath string
		fs         Filesystem
		logger     *lagertest.TestLogger
	)

	BeforeEach(func() {
		parentDir = GinkgoT().TempDir()
		importPath = GinkgoT().TempDir()
		logger = lagertest.NewTestLogger("fs")
		driver = volumefakes.FakeDriver{}
		f, err := NewFilesystem(logger, &driver, parentDir)
		Expect(err).NotTo(HaveOccurred())
		fs = f

		driver.CreateVolumeStub = func(fiv FilesystemInitVolume) error {
			return os.Mkdir(fiv.DataPath(), 0755)
		}
	})

	It("creates the volume", func() {
		f, err := os.Create(filepath.Join(importPath, "some-file"))
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()

		is := ImportStrategy{Path: importPath}
		ivol, err := is.Materialize(logger, "new-vol", fs, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(ivol).ToNot(BeNil())
		Expect(filepath.Join(ivol.DataPath(), "some-file")).To(BeARegularFile(), "should contain the file from the import path")
	})

	It("destroys the volume if it errors", func() {
		fakeTar := filepath.Join(importPath, "some-file.tgz")
		f, err := os.Create(fakeTar)
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()

		driver.DestroyVolumeReturns(errors.New("destroy-volume-err"))

		streamer := volumefakes.FakeStreamer{}
		streamer.InReturns(false, errors.New("streamer-err"))

		is := ImportStrategy{Path: fakeTar}
		ivol, err := is.Materialize(logger, "new-vol", fs, &streamer)
		Expect(err).To(HaveOccurred())
		Expect(ivol).To(BeNil())

		Expect(driver.DestroyVolumeCallCount()).To(Equal(1), "should call volume.Destroy()")
		logs := logger.Logs()
		Expect(len(logs)).To(Equal(4))
		Expect(logs[1].Message).To(Equal("fs.failed-to-stream-in"), "should log the streamer.In() error")
		Expect(logs[1].LogLevel).To(Equal(lager.ERROR))
		Expect(logs[1].Data["error"]).To(Equal("streamer-err"))

		Expect(logs[3].Message).To(Equal("fs.filesystem.driver-destroy-volume"), "should log the volume.Destroy() error")
		Expect(logs[3].LogLevel).To(Equal(lager.ERROR))
		Expect(logs[3].Data["error"]).To(Equal("destroy-volume-err"))
	})
})
