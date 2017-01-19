package exec_test

import (
	"archive/tar"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng/dbngfakes"
	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/execfakes"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/resource/resourcefakes"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("DependentGet", func() {
	var (
		fakeWorkerClient           *workerfakes.FakeClient
		fakeResourceFetcher        *resourcefakes.FakeFetcher
		fakeDBResourceCacheFactory *dbngfakes.FakeResourceCacheFactory
		fakeVersionedSource        *resourcefakes.FakeVersionedSource
		fakeFetchSource            *resourcefakes.FakeFetchSource
		getDelegate                *execfakes.FakeGetDelegate
		resourceConfig             atc.ResourceConfig
		params                     atc.Params
		version                    atc.Version
		tags                       []string
		resourceTypes              atc.ResourceTypes

		inStep *execfakes.FakeStep
		repo   *worker.ArtifactRepository

		step    Step
		process ifrit.Process

		factory Factory

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		identifier = worker.Identifier{
			BuildID: 1234,
			PlanID:  atc.PlanID("some-plan-id"),
		}
		workerMetadata = worker.Metadata{
			PipelineID:   4567,
			PipelineName: "some-pipeline",
			Type:         db.ContainerTypeGet,
			StepName:     "some-step",
		}

		stepMetadata testMetadata = []string{"a=1", "b=2"}

		sourceName worker.ArtifactName = "some-source-name"

		teamID int = 123
	)

	BeforeEach(func() {
		fakeWorkerClient = new(workerfakes.FakeClient)
		fakeResourceFetcher = new(resourcefakes.FakeFetcher)
		fakeResourceFactory := new(resourcefakes.FakeResourceFactory)
		fakeDBResourceCacheFactory = new(dbngfakes.FakeResourceCacheFactory)

		factory = NewGardenFactory(fakeWorkerClient, fakeResourceFetcher, fakeResourceFactory, fakeDBResourceCacheFactory)

		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()

		getDelegate = new(execfakes.FakeGetDelegate)
		getDelegate.StdoutReturns(stdoutBuf)
		getDelegate.StderrReturns(stderrBuf)

		resourceConfig = atc.ResourceConfig{
			Name:   "some-resource",
			Type:   "some-resource-type",
			Source: atc.Source{"some": "source"},
		}

		params = atc.Params{"some-param": "some-value"}

		version = atc.Version{"some-version": "some-value"}

		tags = []string{"some", "tags"}

		resourceTypes = atc.ResourceTypes{
			{
				Name:   "custom-resource",
				Type:   "custom-type",
				Source: atc.Source{"some-custom": "source"},
			},
		}

		inStep = new(execfakes.FakeStep)
		inStep.ResultStub = func(x interface{}) bool {
			switch v := x.(type) {
			case *VersionInfo:
				*v = VersionInfo{
					Version: version,
				}
				return true

			default:
				return false
			}
		}

		repo = worker.NewArtifactRepository()

		fakeVersionedSource = new(resourcefakes.FakeVersionedSource)
		fakeFetchSource = new(resourcefakes.FakeFetchSource)
		fakeFetchSource.VersionedSourceReturns(fakeVersionedSource)
	})

	JustBeforeEach(func() {
		step = factory.DependentGet(
			lagertest.NewTestLogger("test"),
			stepMetadata,
			sourceName,
			identifier,
			workerMetadata,
			getDelegate,
			resourceConfig,
			tags,
			teamID,
			params,
			resourceTypes,
		).Using(inStep, repo)

		process = ifrit.Invoke(step)
	})

	Context("before initializing the resource", func() {
		var callCountDuringInit chan int

		BeforeEach(func() {
			callCountDuringInit = make(chan int, 1)
			fakeVersionedSource.VersionReturns(atc.Version{"some": "version"})
			fakeVersionedSource.MetadataReturns([]atc.MetadataField{{"some", "metadata"}})

			fakeResourceFetcher.FetchStub = func(
				lager.Logger,
				resource.Session,
				atc.Tags,
				int,
				atc.ResourceTypes,
				resource.ResourceInstance,
				resource.Metadata,
				worker.ImageFetchingDelegate,
				resource.ResourceOptions,
				<-chan os.Signal,
				chan<- struct{},
			) (resource.FetchSource, error) {
				callCountDuringInit <- getDelegate.InitializingCallCount()
				return fakeFetchSource, nil
			}
		})

		It("calls the Initializing method on the delegate", func() {
			Expect(<-callCountDuringInit).To(Equal(1))
		})
	})

	Context("when the tracker can initialize the resource", func() {
		BeforeEach(func() {
			fakeResourceFetcher.FetchReturns(fakeFetchSource, nil)
			fakeFetchSource.VersionedSourceReturns(fakeVersionedSource)

			fakeVersionedSource.VersionReturns(atc.Version{"some": "version"})
			fakeVersionedSource.MetadataReturns([]atc.MetadataField{{"some", "metadata"}})
		})

		It("initializes the resource with the correct type and session id, making sure that it is not ephemeral", func() {
			Expect(fakeResourceFetcher.FetchCallCount()).To(Equal(1))
			_, sid, tags, actualTeamID, actualResourceTypes, cacheID, sm, delegate, resourceOptions, _, _ := fakeResourceFetcher.FetchArgsForCall(0)
			Expect(sm).To(Equal(stepMetadata))
			Expect(sid).To(Equal(resource.Session{
				ID: worker.Identifier{
					BuildID: 1234,
					PlanID:  atc.PlanID("some-plan-id"),
					Stage:   db.ContainerStageRun,
				},
				Metadata:  workerMetadata,
				Ephemeral: false,
			}))
			Expect(tags).To(ConsistOf("some", "tags"))
			Expect(actualTeamID).To(Equal(teamID))
			Expect(cacheID).To(Equal(resource.NewBuildResourceInstance(
				"some-resource-type",
				version,
				resourceConfig.Source,
				params,
				1234,
				4567,
				resourceTypes,
				fakeDBResourceCacheFactory,
			)))
			Expect(actualResourceTypes).To(Equal(
				atc.ResourceTypes{
					{
						Name:   "custom-resource",
						Type:   "custom-type",
						Source: atc.Source{"some-custom": "source"},
					},
				}))
			Expect(delegate).To(Equal(getDelegate))
			Expect(resourceOptions.ResourceType()).To(Equal(resource.ResourceType("some-resource-type")))
			expectedLockName := fmt.Sprintf("%x",
				sha256.Sum256([]byte(
					`{"type":"some-resource-type","version":{"some-version":"some-value"},"source":{"some":"source"},"params":{"some-param":"some-value"},"worker_name":"fake-worker-name"}`,
				)),
			)
			Expect(resourceOptions.LockName("fake-worker-name")).To(Equal(expectedLockName))
		})

		Describe("the source registered with the repository", func() {
			var artifactSource worker.ArtifactSource

			JustBeforeEach(func() {
				Eventually(process.Wait()).Should(Receive(BeNil()))

				var found bool
				artifactSource, found = repo.SourceFor(sourceName)
				Expect(found).To(BeTrue())
			})

			Describe("streaming to a destination", func() {
				var fakeDestination *workerfakes.FakeArtifactDestination

				BeforeEach(func() {
					fakeDestination = new(workerfakes.FakeArtifactDestination)
				})

				Context("when the resource can stream out", func() {
					var (
						streamedOut io.ReadCloser
					)

					BeforeEach(func() {
						streamedOut = gbytes.NewBuffer()
						fakeVersionedSource.StreamOutReturns(streamedOut, nil)
					})

					It("streams the resource to the destination", func() {
						err := artifactSource.StreamTo(fakeDestination)
						Expect(err).NotTo(HaveOccurred())

						Expect(fakeVersionedSource.StreamOutCallCount()).To(Equal(1))
						Expect(fakeVersionedSource.StreamOutArgsForCall(0)).To(Equal("."))

						Expect(fakeDestination.StreamInCallCount()).To(Equal(1))
						dest, src := fakeDestination.StreamInArgsForCall(0)
						Expect(dest).To(Equal("."))
						Expect(src).To(Equal(streamedOut))
					})

					Context("when streaming out of the versioned source fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeVersionedSource.StreamOutReturns(nil, disaster)
						})

						It("returns the error", func() {
							Expect(artifactSource.StreamTo(fakeDestination)).To(Equal(disaster))
						})
					})

					Context("when streaming in to the destination fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeDestination.StreamInReturns(disaster)
						})

						It("returns the error", func() {
							Expect(artifactSource.StreamTo(fakeDestination)).To(Equal(disaster))
						})
					})
				})

				Context("when the resource cannot stream out", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeVersionedSource.StreamOutReturns(nil, disaster)
					})

					It("returns the error", func() {
						Expect(artifactSource.StreamTo(fakeDestination)).To(Equal(disaster))
					})
				})
			})

			Describe("streaming a file out", func() {
				Context("when the resource can stream out", func() {
					var (
						fileContent = "file-content"

						tarBuffer *gbytes.Buffer
					)

					BeforeEach(func() {
						tarBuffer = gbytes.NewBuffer()
						fakeVersionedSource.StreamOutReturns(tarBuffer, nil)
					})

					Context("when the file exists", func() {
						BeforeEach(func() {
							tarWriter := tar.NewWriter(tarBuffer)

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
							reader, err := artifactSource.StreamFile("some-path")
							Expect(err).NotTo(HaveOccurred())

							Expect(ioutil.ReadAll(reader)).To(Equal([]byte(fileContent)))

							Expect(fakeVersionedSource.StreamOutArgsForCall(0)).To(Equal("some-path"))
						})

						Describe("closing the stream", func() {
							It("closes the stream from the versioned source", func() {
								reader, err := artifactSource.StreamFile("some-path")
								Expect(err).NotTo(HaveOccurred())

								Expect(tarBuffer.Closed()).To(BeFalse())

								err = reader.Close()
								Expect(err).NotTo(HaveOccurred())

								Expect(tarBuffer.Closed()).To(BeTrue())
							})
						})
					})

					Context("but the stream is empty", func() {
						It("returns ErrFileNotFound", func() {
							_, err := artifactSource.StreamFile("some-path")
							Expect(err).To(MatchError(FileNotFoundError{Path: "some-path"}))
						})
					})
				})

				Context("when the resource cannot stream out", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeVersionedSource.StreamOutReturns(nil, disaster)
					})

					It("returns the error", func() {
						_, err := artifactSource.StreamFile("some-path")
						Expect(err).To(Equal(disaster))
					})
				})
			})
		})
	})

	Context("when the tracker fails to initialize the resource", func() {
		disaster := errors.New("nope")

		BeforeEach(func() {
			fakeResourceFetcher.FetchReturns(nil, disaster)
		})

		It("exits with the failure", func() {
			Eventually(process.Wait()).Should(Receive(Equal(disaster)))
		})

		It("invokes the delegate's Failed callback", func() {
			Eventually(process.Wait()).Should(Receive(Equal(disaster)))

			Expect(getDelegate.CompletedCallCount()).To(BeZero())

			Expect(getDelegate.FailedCallCount()).To(Equal(1))
			Expect(getDelegate.FailedArgsForCall(0)).To(Equal(disaster))
		})
	})
})
