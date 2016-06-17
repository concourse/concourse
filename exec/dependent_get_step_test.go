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
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("GardenFactory", func() {
	var (
		fakeTracker        *rfakes.FakeTracker
		fakeTrackerFactory *fakes.FakeTrackerFactory
		fakeWorkerClient   *wfakes.FakeClient

		factory Factory

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		identifier = worker.Identifier{
			BuildID: 1234,
			PlanID:  atc.PlanID("some-plan-id"),
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
		fakeTracker = new(rfakes.FakeTracker)
		fakeTrackerFactory = new(fakes.FakeTrackerFactory)

		fakeWorkerClient = new(wfakes.FakeClient)

		factory = NewGardenFactory(fakeWorkerClient, fakeTracker)

		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
	})

	Describe("DependentGet", func() {
		var (
			getDelegate    *fakes.FakeGetDelegate
			resourceConfig atc.ResourceConfig
			params         atc.Params
			version        atc.Version
			tags           []string
			resourceTypes  atc.ResourceTypes

			satisfiedWorker *wfakes.FakeWorker

			inStep *fakes.FakeStep
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

			inStep = new(fakes.FakeStep)
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

			repo = NewSourceRepository()
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
				params,
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

			It("initializes the resource with the correct type and session id, making sure that it is not ephemeral", func() {
				Expect(fakeTracker.InitWithCacheCallCount()).To(Equal(1))

				_, sm, sid, typ, tags, cacheID, actualResourceTypes, delegate := fakeTracker.InitWithCacheArgsForCall(0)
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
				Expect(typ).To(Equal(resource.ResourceType("some-resource-type")))
				Expect(tags).To(ConsistOf("some", "tags"))
				Expect(cacheID).To(Equal(resource.ResourceCacheIdentifier{
					Type:    "some-resource-type",
					Source:  resourceConfig.Source,
					Params:  params,
					Version: version,
				}))
				Expect(actualResourceTypes).To(Equal(
					atc.ResourceTypes{
						{
							Name:   "custom-resource",
							Type:   "custom-type",
							Source: atc.Source{"some-custom": "source"},
						},
					}))
				Expect(delegate).To(Equal(getDelegate))
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

			It("runs the get resource action", func() {
				Expect(fakeVersionedSource.RunCallCount()).To(Equal(1))
			})

			It("reports the fetched version info", func() {
				var info VersionInfo
				Expect(step.Result(&info)).To(BeTrue())
				Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
				Expect(info.Metadata).To(Equal([]atc.MetadataField{{"some", "metadata"}}))
			})

			It("is successful", func() {
				Eventually(process.Wait()).Should(Receive(BeNil()))

				var success Success
				Expect(step.Result(&success)).To(BeTrue())
				Expect(bool(success)).To(BeTrue())
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

			Context("when fetching fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeVersionedSource.RunReturns(disaster)
				})

				It("exits with the failure", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
				})

				It("releases the resource with the containerFailureTTL", func() {
					<-process.Wait()

					Expect(fakeResource.ReleaseCallCount()).To(BeZero())

					step.Release()
					Expect(fakeResource.ReleaseCallCount()).To(Equal(1))
					Expect(fakeResource.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(worker.FinishedContainerTTL)))
				})

				It("invokes the delegate's Failed callback without completing", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))

					Expect(getDelegate.CompletedCallCount()).To(BeZero())

					Expect(getDelegate.FailedCallCount()).To(Equal(1))
					Expect(getDelegate.FailedArgsForCall(0)).To(Equal(disaster))
				})

				Context("with a resource script failure", func() {
					var resourceScriptError resource.ErrResourceScriptFailed

					BeforeEach(func() {
						resourceScriptError = resource.ErrResourceScriptFailed{
							ExitStatus: 1,
						}

						fakeVersionedSource.RunReturns(resourceScriptError)
					})

					It("invokes the delegate's Finished callback instead of failed", func() {
						Eventually(process.Wait()).Should(Receive())

						Expect(getDelegate.FailedCallCount()).To(BeZero())

						Expect(getDelegate.CompletedCallCount()).To(Equal(1))

						status, versionInfo := getDelegate.CompletedArgsForCall(0)
						Expect(status).To(Equal(ExitStatus(1)))
						Expect(versionInfo).To(BeNil())
					})

					It("is not successful", func() {
						Eventually(process.Wait()).Should(Receive(BeNil()))
						Expect(getDelegate.CompletedCallCount()).To(Equal(1))

						var success Success

						Expect(step.Result(&success)).To(BeTrue())
						Expect(bool(success)).To(BeFalse())
					})
				})
			})

			Describe("releasing", func() {
				It("releases the resource with the original container TTL", func() {
					Expect(fakeResource.ReleaseCallCount()).To(BeZero())

					step.Release()
					Expect(fakeResource.ReleaseCallCount()).To(Equal(1))
					Expect(fakeResource.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(worker.FinishedContainerTTL)))
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
