package worker_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/compression/compressionfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/runtime/runtimefakes"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ArtifactSourcer", func() {
	var (
		logger          *lagertest.TestLogger
		fakeCompression *compressionfakes.FakeCompression
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeCompression = new(compressionfakes.FakeCompression)
	})

	It("locates images by handle", func() {
		artifact := runtime.GetArtifact{VolumeHandle: "image"}
		vf := FakeVolumeFinder{Volumes: map[string]worker.Volume{
			"image": newVolumeWithContent(content{".": []byte("image content")}),
		}}

		sourcer := worker.NewArtifactSourcer(fakeCompression, vf, false, 0)
		source, err := sourcer.SourceImage(logger, artifact)
		Expect(err).ToNot(HaveOccurred())

		Expect(source).To(BeStreamableWithContent(content{".": []byte("image content")}))
	})

	It("locates inputs and caches", func() {
		inputs := map[string]runtime.Artifact{
			"existing_cache": &runtime.CacheArtifact{
				TeamID:   1,
				JobID:    1,
				StepName: "task1",
				Path:     "existing_cache",
			},
			"missing_cache": &runtime.CacheArtifact{
				TeamID:   1,
				JobID:    1,
				StepName: "task2",
				Path:     "missing_cache",
			},
			"task_artifact": &runtime.TaskArtifact{VolumeHandle: "output"},
		}
		fakeWorker := new(workerfakes.FakeWorker)
		fakeWorker.FindVolumeForTaskCacheStub = func(_ lager.Logger, teamID int, jobID int, stepName string, path string) (worker.Volume, bool, error) {
			switch path {
			case "existing_cache":
				return new(workerfakes.FakeVolume), true, nil
			case "missing_cache":
				return nil, false, nil
			default:
				return nil, false, fmt.Errorf("unexpected path %s", path)
			}
		}
		vf := FakeVolumeFinder{Volumes: map[string]worker.Volume{
			"output": newVolumeWithContent(content{".": []byte("output")})},
		}

		sourcer := worker.NewArtifactSourcer(fakeCompression, vf, false, 0)
		inputSources, err := sourcer.SourceInputsAndCaches(logger, 0, inputs)
		Expect(err).ToNot(HaveOccurred())

		sources := make([]worker.ArtifactSource, len(inputSources))
		for i, v := range inputSources {
			sources[i] = v.Source()
		}

		Expect(sources).To(ConsistOf(
			BeStreamableWithContent(content{".": []byte("output")}),
			ExistOnWorker(fakeWorker),
			Not(ExistOnWorker(fakeWorker)),
		))
	})
})

var _ = Describe("StreamableArtifactSource", func() {
	var (
		fakeDestination *workerfakes.FakeArtifactDestination
		fakeVolume      *workerfakes.FakeVolume
		fakeArtifact    *runtimefakes.FakeArtifact

		enabledP2pStreaming bool
		p2pStreamingTimeout time.Duration

		artifactSource worker.StreamableArtifactSource
		comp           compression.Compression
		testLogger     lager.Logger

		disaster error
	)

	BeforeEach(func() {
		fakeArtifact = new(runtimefakes.FakeArtifact)
		fakeVolume = new(workerfakes.FakeVolume)
		fakeDestination = new(workerfakes.FakeArtifactDestination)
		comp = compression.NewGzipCompression()

		enabledP2pStreaming = false
		p2pStreamingTimeout = 15 * time.Minute

		testLogger = lager.NewLogger("test")
		disaster = errors.New("disaster")
	})

	JustBeforeEach(func() {
		artifactSource = worker.NewStreamableArtifactSource(fakeArtifact, fakeVolume, comp, enabledP2pStreaming, p2pStreamingTimeout)
	})

	Context("StreamTo", func() {
		var streamToErr error

		JustBeforeEach(func() {
			streamToErr = artifactSource.StreamTo(context.TODO(), fakeDestination)
		})

		Context("via atc", func() {
			var outStream *gbytes.Buffer

			BeforeEach(func() {
				outStream = gbytes.NewBuffer()
				fakeVolume.StreamOutReturns(outStream, nil)
			})

			Context("when ArtifactSource can successfully stream to ArtifactDestination", func() {

				It("calls StreamOut and StreamIn with the correct params", func() {
					Expect(fakeVolume.StreamOutCallCount()).To(Equal(1))

					_, actualPath, encoding := fakeVolume.StreamOutArgsForCall(0)
					Expect(actualPath).To(Equal("."))
					Expect(encoding).To(Equal(baggageclaim.GzipEncoding))

					_, actualPath, encoding, actualStreamedOutBits := fakeDestination.StreamInArgsForCall(0)
					Expect(actualPath).To(Equal("."))
					Expect(actualStreamedOutBits).To(Equal(outStream))
					Expect(encoding).To(Equal(baggageclaim.GzipEncoding))
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

		Context("p2p", func() {
			BeforeEach(func() {
				enabledP2pStreaming = true
			})

			Context("GetStreamInP2pUrl fails", func() {
				BeforeEach(func() {
					fakeDestination.GetStreamInP2pUrlReturns("", disaster)
				})

				It("does return an err", func() {
					Expect(streamToErr).To(HaveOccurred())
					Expect(streamToErr).To(Equal(disaster))
				})

				It("should not call StreamP2pOut", func() {
					Expect(fakeDestination.GetStreamInP2pUrlCallCount()).To(Equal(1))

					_, actualPath := fakeDestination.GetStreamInP2pUrlArgsForCall(0)
					Expect(actualPath).To(Equal("."))

					Expect(fakeVolume.StreamP2pOutCallCount()).To(Equal(0))
				})
			})

			Context("GetStreamInP2pUrl succeeds", func() {
				BeforeEach(func() {
					fakeDestination.GetStreamInP2pUrlReturns("some-url", nil)
				})

				It("calls GetStreamInP2pUrl and StreamP2pOut with the correct params", func() {
					Expect(fakeDestination.GetStreamInP2pUrlCallCount()).To(Equal(1))

					_, actualPath := fakeDestination.GetStreamInP2pUrlArgsForCall(0)
					Expect(actualPath).To(Equal("."))

					Expect(fakeVolume.StreamP2pOutCallCount()).To(Equal(1))

					_, actualPath, actualStreamUrl, actualEncoding := fakeVolume.StreamP2pOutArgsForCall(0)
					Expect(actualPath).To(Equal("."))
					Expect(actualStreamUrl).To(Equal("some-url"))
					Expect(actualEncoding).To(Equal(baggageclaim.GzipEncoding))
				})

				Context("StreamP2pOut fails", func() {
					BeforeEach(func() {
						fakeVolume.StreamP2pOutReturns(disaster)
					})

					It("does return an err", func() {
						Expect(streamToErr).To(HaveOccurred())
						Expect(streamToErr).To(Equal(disaster))
					})
				})

				Context("StreamP2pOut succeeds", func() {
					It("does not return an err", func() {
						Expect(streamToErr).ToNot(HaveOccurred())
					})
				})
			})
		})
	})

	Context("StreamFile", func() {
		var (
			streamFileErr    error
			streamFileReader io.ReadCloser
		)

		JustBeforeEach(func() {
			streamFileReader, streamFileErr = artifactSource.StreamFile(context.TODO(), "some-file")
		})

		Context("when ArtifactSource can successfully stream a file out", func() {
			var (
				fileContent = "file-content"
				tgzBuffer   *gbytes.Buffer
			)

			BeforeEach(func() {
				tgzBuffer = gbytes.NewBuffer()
				fakeVolume.StreamOutReturns(tgzBuffer, nil)
				gzipWriter := gzip.NewWriter(tgzBuffer)
				defer gzipWriter.Close()

				tarWriter := tar.NewWriter(gzipWriter)
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
				_, path, encoding := fakeVolume.StreamOutArgsForCall(0)
				Expect(path).To(Equal("some-file"))
				Expect(encoding).To(Equal(baggageclaim.GzipEncoding))
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
		})
	})

	Context("ExistsOn", func() {
		var (
			fakeWorker   *workerfakes.FakeWorker
			actualVolume worker.Volume
			actualFound  bool
			actualErr    error
		)

		BeforeEach(func() {
			fakeWorker = new(workerfakes.FakeWorker)
			fakeWorker.LookupVolumeReturns(fakeVolume, true, disaster)
			fakeArtifact.IDReturns("some-id")
		})

		JustBeforeEach(func() {
			actualVolume, actualFound, actualErr = artifactSource.ExistsOn(testLogger, fakeWorker)
		})

		Context("when the volume belongs to the worker passed in", func() {
			BeforeEach(func() {
				fakeWorker.NameReturns("some-foo-worker-name")
				fakeVolume.WorkerNameReturns("some-foo-worker-name")
			})
			It("returns the volume", func() {
				Expect(actualFound).To(BeTrue())
				Expect(actualVolume).To(Equal(fakeVolume))
				Expect(actualErr).ToNot(HaveOccurred())
			})
		})
		Context("when the volume doesn't belong to the worker passed in", func() {
			BeforeEach(func() {
				fakeWorker.NameReturns("some-foo-worker-name")
				fakeVolume.WorkerNameReturns("some-other-foo-worker-name")
			})
			Context("when the volume has a resource cache", func() {
				var fakeResourceCache db.ResourceCache

				BeforeEach(func() {
					fakeResourceCache = new(dbfakes.FakeResourceCache)
					fakeWorker.FindResourceCacheForVolumeReturns(fakeResourceCache, true, nil)

				})

				It("queries the worker's local volume for the resourceCache", func() {
					_, actualResourceCache := fakeWorker.FindVolumeForResourceCacheArgsForCall(0)
					Expect(actualResourceCache).To(Equal(fakeResourceCache))
				})

				Context("when the resource cache has a local volume on the worker", func() {
					var localFakeVolume worker.Volume
					BeforeEach(func() {
						localFakeVolume = new(workerfakes.FakeVolume)
						fakeWorker.FindVolumeForResourceCacheReturns(localFakeVolume, true, nil)
					})
					It("returns worker's local volume for the resourceCache", func() {
						Expect(actualFound).To(BeTrue())
						Expect(actualVolume).To(Equal(localFakeVolume))
						Expect(actualErr).ToNot(HaveOccurred())
					})
				})

			})

			Context("when the volume does NOT have a resource cache", func() {
				BeforeEach(func() {
					fakeWorker.FindResourceCacheForVolumeReturns(nil, false, nil)

				})
				It("returns not found", func() {
					Expect(actualFound).To(BeFalse())
					Expect(actualErr).ToNot(HaveOccurred())
				})
			})
		})
	})

})

var _ = Describe("CacheArtifactSource", func() {
	Context("ExistsOn", func() {
		var (
			fakeWorker          *workerfakes.FakeWorker
			actualVolume        worker.Volume
			actualFound         bool
			actualErr           error
			disaster            error
			fakeVolume          *workerfakes.FakeVolume
			cacheArtifactSource worker.ArtifactSource
			cacheArtifact       runtime.CacheArtifact
			testLogger          lager.Logger
		)

		BeforeEach(func() {
			fakeWorker = new(workerfakes.FakeWorker)
			fakeVolume = new(workerfakes.FakeVolume)
			disaster = errors.New("disaster")

			fakeWorker.FindVolumeForTaskCacheReturns(fakeVolume, true, disaster)

			testLogger = lager.NewLogger("cacheArtifactSource")

			cacheArtifact = runtime.CacheArtifact{
				TeamID:   5,
				JobID:    18,
				StepName: "some-step-name",
				Path:     "some/path/foo",
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

		It("returns the response of Worker.FindVolumeForTaskCache", func() {
			Expect(actualVolume).To(Equal(fakeVolume))
			Expect(actualFound).To(BeTrue())
			Expect(actualErr).To(Equal(disaster))
		})
	})
})
