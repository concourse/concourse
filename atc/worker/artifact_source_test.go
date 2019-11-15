package worker_test

import (
	"archive/tar"
	"context"
	"io"
	"io/ioutil"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"code.cloudfoundry.org/lager"
	"github.com/DataDog/zstd"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/runtime/runtimefakes"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
)

var _ = Describe("StreamableArtifactSource", func() {
	var (
		fakeDestination *workerfakes.FakeArtifactDestination
		fakeVolume      *workerfakes.FakeVolume
		fakeArtifact    *runtimefakes.FakeArtifact

		artifactSource worker.StreamableArtifactSource
		testLogger     lager.Logger

		disaster error
	)

	BeforeEach(func() {
		fakeArtifact = new(runtimefakes.FakeArtifact)
		fakeVolume = new(workerfakes.FakeVolume)
		fakeDestination = new(workerfakes.FakeArtifactDestination)

		artifactSource = worker.NewStreamableArtifactSource(fakeArtifact, fakeVolume)
		testLogger = lager.NewLogger("test")
		disaster = errors.New("disaster")
	})

	Context("StreamTo", func() {
		var (
			streamToErr error
			outStream *gbytes.Buffer
		)

		BeforeEach(func() {
			outStream = gbytes.NewBuffer()
			fakeVolume.StreamOutReturns(outStream, nil)
		})

		JustBeforeEach(func() {
			streamToErr = artifactSource.StreamTo(context.TODO(), testLogger, fakeDestination)
		})

		Context("when ArtifactSource can successfully stream to ArtifactDestination", func() {

			It("calls StreamOut and StreamIn with the correct params", func() {
				Expect(fakeVolume.StreamOutCallCount()).To(Equal(1))

				_, actualPath := fakeVolume.StreamOutArgsForCall(0)
				Expect(actualPath).To(Equal("."))

				_, actualPath, actualStreamedOutBits := fakeDestination.StreamInArgsForCall(0)
				Expect(actualPath).To(Equal("."))
				Expect(actualStreamedOutBits).To(Equal(outStream))
			})

			It("does not return an err", func() {
				Expect(streamToErr).ToNot(HaveOccurred())
			})
		})

		Context("when streaming out of source fails ", func() {
			BeforeEach(func() {
				fakeVolume.StreamOutReturns(nil, disaster)
			})
			It("returns the err", func() {
				Expect(streamToErr).To(Equal(disaster))
			})
		})

		Context("when streaming in to destination fails ", func() {
			BeforeEach(func() {
				fakeDestination.StreamInReturns(disaster)
			})
			It("returns the err", func() {
				Expect(streamToErr).To(Equal(disaster))
			})
			It("closes the streamOut io.reader", func() {
				Expect(outStream.Closed()).To(BeTrue())
			})
		})
	})

	Context("StreamFile", func() {
		var (
			streamFileErr error
			streamFileReader io.ReadCloser
		)

		JustBeforeEach(func() {
			streamFileReader, streamFileErr = artifactSource.StreamFile(context.TODO(), testLogger, "some-file")
		})

		Context("when ArtifactSource can successfully stream a file out", func() {
			var (
				fileContent = "file-content"
				tgzBuffer   *gbytes.Buffer
			)

			BeforeEach(func() {
				tgzBuffer = gbytes.NewBuffer()
				fakeVolume.StreamOutReturns(tgzBuffer, nil)
				zstdWriter := zstd.NewWriter(tgzBuffer)
				defer zstdWriter.Close()

				tarWriter := tar.NewWriter(zstdWriter)
				defer tarWriter.Close()

				err := tarWriter.WriteHeader(&tar.Header{
					Name: "some-file",
					Mode: 0644,
					Size: int64(len(fileContent)),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = tarWriter.Write([]byte(fileContent))
				Expect(err).NotTo(HaveOccurred())
			})

			It("streams out the given path", func() {
				Expect(streamFileErr).NotTo(HaveOccurred())

				Expect(ioutil.ReadAll(streamFileReader)).To(Equal([]byte(fileContent)))
				_, path := fakeVolume.StreamOutArgsForCall(0)
				Expect(path).To(Equal("some-file"))
			})

			It("closes the stream from the volume", func() {
				Expect(streamFileErr).NotTo(HaveOccurred())

				Expect(tgzBuffer.Closed()).To(BeFalse())

				err := streamFileReader.Close()
				Expect(err).NotTo(HaveOccurred())

				Expect(tgzBuffer.Closed()).To(BeTrue())
			})
		})

		Context("when ArtifactSource fails to stream a file out", func() {

			Context("when streaming out of source fails ", func() {
				BeforeEach(func() {
					fakeVolume.StreamOutReturns(nil, disaster)
				})

				It("returns the error", func() {
					Expect(streamFileErr).To(Equal(disaster))
				})

			})

			Context("when the file is not found in the tar archive", func() {
				var (
					tgzBuffer   *gbytes.Buffer
				)
				BeforeEach(func() {
					tgzBuffer = gbytes.NewBuffer()
					fakeVolume.StreamOutReturns(tgzBuffer, nil)
				})

				It("returns ErrFileNotFound", func() {
					Expect(streamFileErr).To(MatchError(runtime.FileNotFoundError{Path: "some-file"}))
				})
			})
		})
	})


	Context("ExistsOn", func() {
		var (
			fakeWorker *workerfakes.FakeWorker
			actualVolume worker.Volume
			actualFound bool
			actualErr error
		)

		BeforeEach(func() {
			fakeWorker = new(workerfakes.FakeWorker)
			fakeWorker.LookupVolumeReturns(fakeVolume, true, disaster)
			fakeArtifact.IDReturns("some-id")
		})

		JustBeforeEach(func() {
			actualVolume, actualFound, actualErr = artifactSource.ExistsOn(testLogger, fakeWorker)
		})

		It("calls Worker.LookupVolume with the the correct params", func() {
			_, actualArtifactID := fakeWorker.LookupVolumeArgsForCall(0)
			Expect(actualArtifactID).To(Equal(fakeArtifact.ID()))
		})

		It("returns the response of Worker.LookupVolume", func(){
			Expect(actualVolume).To(Equal(fakeVolume))
			Expect(actualFound).To(BeTrue())
			Expect(actualErr).To(Equal(disaster))

		})
	})

})

var _ = Describe("CacheArtifactSource", func() {
	Context("ExistsOn", func() {
		var (
			fakeWorker *workerfakes.FakeWorker
			actualVolume worker.Volume
			actualFound bool
			actualErr error
			disaster error
			fakeVolume      *workerfakes.FakeVolume
			cacheArtifactSource worker.ArtifactSource
			cacheArtifact runtime.CacheArtifact
			testLogger     lager.Logger
		)


		BeforeEach(func() {
			fakeWorker = new(workerfakes.FakeWorker)
			fakeVolume = new(workerfakes.FakeVolume)
			disaster = errors.New("disaster")

			fakeWorker.FindVolumeForTaskCacheReturns(fakeVolume, true, disaster)

			testLogger = lager.NewLogger("cacheArtifactSource")

			cacheArtifact = runtime.CacheArtifact{
				TeamID: 5,
				JobID: 18,
				StepName: "some-step-name",
				Path : "some/path/foo",
			}
			cacheArtifactSource = worker.NewCacheArtifactSource(cacheArtifact)

		})

		JustBeforeEach(func() {
			actualVolume, actualFound, actualErr = cacheArtifactSource.ExistsOn(testLogger, fakeWorker)
		})

		It("calls Worker.FindVolumeForTaskCache with the the correct params", func() {
			_, actualTeamID, actualJobID, actualStepName, actualPath := fakeWorker.FindVolumeForTaskCacheArgsForCall(0)
			Expect(actualTeamID).To(Equal(cacheArtifact.TeamID))
			Expect(actualJobID).To(Equal(cacheArtifact.JobID))
			Expect(actualStepName).To(Equal(cacheArtifact.StepName))
			Expect(actualPath).To(Equal(cacheArtifact.Path))
		})

		It("returns the response of Worker.FindVolumeForTaskCache", func(){
			Expect(actualVolume).To(Equal(fakeVolume))
			Expect(actualFound).To(BeTrue())
			Expect(actualErr).To(Equal(disaster))

		})
	})
})

