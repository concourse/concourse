package exec_test

import (
	"archive/tar"
	"errors"
	"io"
	"io/ioutil"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/fakes"
	"github.com/concourse/atc/resource"
	rfakes "github.com/concourse/atc/resource/fakes"
	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("GardenFactory", func() {
	var (
		fakeWorkerClient   *wfakes.FakeClient
		fakeTracker        *rfakes.FakeTracker
		fakeTrackerFactory *fakes.FakeTrackerFactory

		factory Factory

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		identifier = worker.Identifier{
			ResourceID: 1234,
		}
		workerMetadata = worker.Metadata{
			PipelineName: "some-pipeline",
			Type:         db.ContainerTypeGet,
			StepName:     "some-step",
		}

		stepMetadata testMetadata = []string{"a=1", "b=2"}

		sourceName SourceName = "some-source-name"
	)

	BeforeEach(func() {
		fakeWorkerClient = new(wfakes.FakeClient)
		fakeTracker = new(rfakes.FakeTracker)
		fakeTrackerFactory = new(fakes.FakeTrackerFactory)

		factory = NewGardenFactory(fakeWorkerClient, fakeTracker)

		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
	})

	Describe("Get", func() {
		var (
			getDelegate    *fakes.FakeGetDelegate
			resourceConfig atc.ResourceConfig
			params         atc.Params
			version        atc.Version
			tags           []string
			resourceTypes  atc.ResourceTypes

			satisfiedWorker *wfakes.FakeWorker

			inStep Step
			repo   *SourceRepository

			step    Step
			process ifrit.Process
		)

		BeforeEach(func() {
			getDelegate = new(fakes.FakeGetDelegate)
			getDelegate.StdoutReturns(stdoutBuf)
			getDelegate.StderrReturns(stderrBuf)

			satisfiedWorker = new(wfakes.FakeWorker)
			fakeWorkerClient.SatisfyingReturns(satisfiedWorker, nil)

			resourceConfig = atc.ResourceConfig{
				Name:   "some-resource",
				Type:   "some-resource-type",
				Source: atc.Source{"some": "source"},
			}

			tags = []string{"some", "tags"}
			params = atc.Params{"some-param": "some-value"}

			version = atc.Version{"some-version": "some-value"}

			inStep = &NoopStep{}
			repo = NewSourceRepository()

			resourceTypes = atc.ResourceTypes{
				{
					Name:   "custom-resource",
					Type:   "custom-type",
					Source: atc.Source{"some-custom": "source"},
				},
			}
		})

		JustBeforeEach(func() {
			step = factory.Get(
				lagertest.NewTestLogger("test"),
				stepMetadata,
				sourceName,
				identifier,
				workerMetadata,
				getDelegate,
				resourceConfig,
				tags,
				params,
				version,
				resourceTypes,
			).Using(inStep, repo)

			process = ifrit.Invoke(step)
		})

		Context("when the tracker can initialize the resource", func() {
			var (
				fakeResource        *rfakes.FakeResource
				fakeCache           *rfakes.FakeCache
				fakeVersionedSource *rfakes.FakeVersionedSource
			)

			BeforeEach(func() {
				fakeResource = new(rfakes.FakeResource)
				fakeCache = new(rfakes.FakeCache)
				fakeTracker.InitWithCacheReturns(fakeResource, fakeCache, nil)

				fakeVersionedSource = new(rfakes.FakeVersionedSource)
				fakeVersionedSource.VersionReturns(atc.Version{"some": "version"})
				fakeVersionedSource.MetadataReturns([]atc.MetadataField{{"some", "metadata"}})

				fakeResource.GetReturns(fakeVersionedSource)
			})

			It("created a cached resource", func() {
				Expect(fakeTracker.InitWithCacheCallCount()).To(Equal(1))
				_, sm, sid, typ, tags, cacheID, actualResourceTypes, delegate := fakeTracker.InitWithCacheArgsForCall(0)
				Expect(sm).To(Equal(stepMetadata))
				Expect(sid).To(Equal(resource.Session{
					ID: worker.Identifier{
						ResourceID: 1234,
						Stage:      db.ContainerStageRun,
					},
					Metadata: worker.Metadata{
						PipelineName:     "some-pipeline",
						Type:             db.ContainerTypeGet,
						StepName:         "some-step",
						WorkingDirectory: "/tmp/build/get",
					},
					Ephemeral: false,
				}))
				Expect(typ).To(Equal(resource.ResourceType("some-resource-type")))
				Expect(tags).To(ConsistOf("some", "tags"))
				Expect(cacheID).To(Equal(resource.ResourceCacheIdentifier{
					Type:    "some-resource-type",
					Source:  resourceConfig.Source,
					Params:  params,
					Version: version,
				}))
				Expect(actualResourceTypes).To(Equal(atc.ResourceTypes{
					{
						Name:   "custom-resource",
						Type:   "custom-type",
						Source: atc.Source{"some-custom": "source"},
					},
				}))
				Expect(delegate).To(Equal(getDelegate))
			})

			Context("before initializing the resource", func() {
				var callCountDuringInit chan int

				BeforeEach(func() {
					callCountDuringInit = make(chan int, 1)

					fakeTracker.InitWithCacheStub = func(lager.Logger, resource.Metadata, resource.Session, resource.ResourceType, atc.Tags, resource.CacheIdentifier, atc.ResourceTypes, worker.ImageFetchingDelegate) (resource.Resource, resource.Cache, error) {
						callCountDuringInit <- getDelegate.InitializingCallCount()
						return fakeResource, fakeCache, nil
					}
				})

				It("calls the Initializing method on the delegate", func() {
					Expect(<-callCountDuringInit).To(Equal(1))
				})
			})

			It("gets the resource with the correct source, params, and version", func() {
				Expect(fakeResource.GetCallCount()).To(Equal(1))

				_, gotSource, gotParams, gotVersion := fakeResource.GetArgsForCall(0)
				Expect(gotSource).To(Equal(resourceConfig.Source))
				Expect(gotParams).To(Equal(params))
				Expect(gotVersion).To(Equal(version))
			})

			It("gets the resource with the io config forwarded", func() {
				Expect(fakeResource.GetCallCount()).To(Equal(1))

				ioConfig, _, _, _ := fakeResource.GetArgsForCall(0)
				Expect(ioConfig.Stdout).To(Equal(stdoutBuf))
				Expect(ioConfig.Stderr).To(Equal(stderrBuf))
			})

			Context("when the cache is not initialized", func() {
				BeforeEach(func() {
					fakeCache.IsInitializedReturns(false, nil)
				})

				It("runs the get resource action", func() {
					Expect(fakeVersionedSource.RunCallCount()).To(Equal(1))
				})

				Describe("releasing", func() {
					It("releases the resource with infinite TTL", func() {
						<-process.Wait()

						Expect(fakeResource.ReleaseCallCount()).To(BeZero())

						step.Release()
						Expect(fakeResource.ReleaseCallCount()).To(Equal(1))
						Expect(fakeResource.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(worker.FinishedContainerTTL)))
					})
				})

				Context("after the 'get' action completes", func() {
					BeforeEach(func() {
						fakeVersionedSource.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
							Expect(fakeCache.InitializeCallCount()).To(Equal(0))
							return nil
						}
					})

					It("exits with no error", func() {
						Expect(<-process.Wait()).To(BeNil())
					})

					It("marks the cache as initialized", func() {
						<-process.Wait()
						Expect(fakeCache.InitializeCallCount()).To(Equal(1))
					})

					It("reports the fetched version info", func() {
						<-process.Wait()

						var info VersionInfo
						Expect(step.Result(&info)).To(BeTrue())
						Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
						Expect(info.Metadata).To(Equal([]atc.MetadataField{{"some", "metadata"}}))
					})

					It("completes via the delegate", func() {
						<-process.Wait()

						Expect(getDelegate.CompletedCallCount()).Should(Equal(1))

						exitStatus, versionInfo := getDelegate.CompletedArgsForCall(0)

						Expect(exitStatus).To(Equal(ExitStatus(0)))
						Expect(versionInfo).To(Equal(&VersionInfo{
							Version:  atc.Version{"some": "version"},
							Metadata: []atc.MetadataField{{"some", "metadata"}},
						}))
					})

					It("is successful", func() {
						<-process.Wait()

						var success Success
						Expect(step.Result(&success)).To(BeTrue())
						Expect(bool(success)).To(BeTrue())
					})
				})

				Context("when the 'get' action fails", func() {
					BeforeEach(func() {
						fakeVersionedSource.RunReturns(resource.ErrResourceScriptFailed{
							ExitStatus: 1,
						})
					})

					It("exits with no error", func() {
						Expect(<-process.Wait()).To(BeNil())
					})

					It("does not mark the cache as initialized", func() {
						<-process.Wait()
						Expect(fakeCache.InitializeCallCount()).To(Equal(0))
					})

					It("completes via the delegate", func() {
						<-process.Wait()

						Expect(getDelegate.CompletedCallCount()).Should(Equal(1))

						exitStatus, versionInfo := getDelegate.CompletedArgsForCall(0)

						Expect(exitStatus).To(Equal(ExitStatus(1)))
						Expect(versionInfo).To(BeNil())
					})

					It("is not successful", func() {
						<-process.Wait()

						var success Success
						Expect(step.Result(&success)).To(BeTrue())
						Expect(bool(success)).To(BeFalse())
					})
				})

				Context("if the 'get' action is interrupted", func() {
					BeforeEach(func() {
						fakeVersionedSource.RunReturns(resource.ErrAborted)
					})

					It("exits with ErrInterrupted", func() {
						Expect(<-process.Wait()).To(Equal(ErrInterrupted))
					})

					It("does not mark the cache as initialized", func() {
						<-process.Wait()

						Expect(fakeCache.InitializeCallCount()).To(Equal(0))
					})

					It("does not complete via the delegate", func() {
						<-process.Wait()

						Expect(getDelegate.CompletedCallCount()).To(Equal(0))
					})

					Describe("releasing", func() {
						It("releases the resource with the configured containerFailureTTL", func() {
							<-process.Wait()

							Expect(fakeResource.ReleaseCallCount()).To(BeZero())

							step.Release()
							Expect(fakeResource.ReleaseCallCount()).To(Equal(1))
							Expect(fakeResource.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(worker.FinishedContainerTTL)))
						})
					})
				})

				Context("when the 'get' action errors", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeVersionedSource.RunReturns(disaster)
					})

					It("exits with the error", func() {
						Expect(<-process.Wait()).To(Equal(disaster))
					})

					It("does not mark the cache as initialized", func() {
						<-process.Wait()

						Expect(fakeCache.InitializeCallCount()).To(Equal(0))
					})

					It("does not complete via the delegate", func() {
						<-process.Wait()

						Expect(getDelegate.CompletedCallCount()).To(Equal(0))
					})

					Describe("releasing", func() {
						It("releases the resource with the configured containerFailureTTL", func() {
							<-process.Wait()

							Expect(fakeResource.ReleaseCallCount()).To(BeZero())

							step.Release()
							Expect(fakeResource.ReleaseCallCount()).To(Equal(1))
							Expect(fakeResource.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(worker.FinishedContainerTTL)))
						})
					})
				})
			})

			Context("when the cache is already initialized", func() {
				BeforeEach(func() {
					fakeCache.IsInitializedReturns(true, nil)
				})

				It("does not run the get resource action", func() {
					Expect(fakeVersionedSource.RunCallCount()).To(Equal(0))
				})

				It("logs a helpful message", func() {
					Expect(stdoutBuf).To(gbytes.Say("using version of resource found in cache\n"))
				})

				It("reports the fetched version info", func() {
					var info VersionInfo
					Expect(step.Result(&info)).To(BeTrue())
					Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
					Expect(info.Metadata).To(Equal([]atc.MetadataField{{"some", "metadata"}}))
				})

				It("completes via the delegate", func() {
					Eventually(getDelegate.CompletedCallCount).Should(Equal(1))

					exitStatus, versionInfo := getDelegate.CompletedArgsForCall(0)

					Expect(exitStatus).To(Equal(ExitStatus(0)))
					Expect(versionInfo).To(Equal(&VersionInfo{
						Version:  atc.Version{"some": "version"},
						Metadata: []atc.MetadataField{{"some", "metadata"}},
					}))
				})

				It("is successful", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))

					var success Success
					Expect(step.Result(&success)).To(BeTrue())
					Expect(bool(success)).To(BeTrue())
				})
			})

			Context("when the cache cannot report whether it's initialized or not", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeCache.IsInitializedReturns(false, disaster)
				})

				It("exits with the error", func() {
					Expect(<-process.Wait()).To(Equal(disaster))
				})

				It("does not run the get action", func() {
					Expect(fakeVersionedSource.RunCallCount()).To(Equal(0))
				})
			})

			Describe("signalling", func() {
				var receivedSignals <-chan os.Signal

				BeforeEach(func() {
					sigs := make(chan os.Signal)
					receivedSignals = sigs

					fakeVersionedSource.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
						close(ready)
						sigs <- <-signals
						return nil
					}
				})

				It("forwards to the resource", func() {
					process.Signal(os.Interrupt)
					Eventually(receivedSignals).Should(Receive(Equal(os.Interrupt)))
					Eventually(process.Wait()).Should(Receive())
				})
			})

			Describe("releasing", func() {
				It("releases the resource", func() {
					Expect(fakeResource.ReleaseCallCount()).To(BeZero())

					step.Release()
					Expect(fakeResource.ReleaseCallCount()).To(Equal(1))
				})
			})

			Describe("the source registered with the repository", func() {
				var artifactSource ArtifactSource

				JustBeforeEach(func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))

					var found bool
					artifactSource, found = repo.SourceFor(sourceName)
					Expect(found).To(BeTrue())
				})

				Describe("streaming to a destination", func() {
					var fakeDestination *fakes.FakeArtifactDestination

					BeforeEach(func() {
						fakeDestination = new(fakes.FakeArtifactDestination)
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
				fakeTracker.InitWithCacheReturns(nil, nil, disaster)
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
})
