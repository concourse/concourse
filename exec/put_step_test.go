package exec_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
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

		stepMetadata testMetadata = []string{"a=1", "b=2"}

		identifier = worker.Identifier{
			ResourceID: 1234,
		}
		workerMetadata = worker.Metadata{
			PipelineName: "some-pipeline",
			Type:         db.ContainerTypePut,
			StepName:     "some-step",
		}
	)

	BeforeEach(func() {
		fakeWorkerClient = new(wfakes.FakeClient)
		fakeTracker = new(rfakes.FakeTracker)
		fakeTrackerFactory = new(fakes.FakeTrackerFactory)

		factory = NewGardenFactory(fakeWorkerClient, fakeTracker)

		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
	})

	Describe("Put", func() {
		var (
			putDelegate    *fakes.FakePutDelegate
			resourceConfig atc.ResourceConfig
			params         atc.Params
			tags           []string
			resourceTypes  atc.ResourceTypes

			inStep *fakes.FakeStep
			repo   *SourceRepository

			step    Step
			process ifrit.Process
		)

		BeforeEach(func() {
			putDelegate = new(fakes.FakePutDelegate)
			putDelegate.StdoutReturns(stdoutBuf)
			putDelegate.StderrReturns(stderrBuf)

			resourceConfig = atc.ResourceConfig{
				Name:   "some-resource",
				Type:   "some-resource-type",
				Source: atc.Source{"some": "source"},
			}

			params = atc.Params{"some-param": "some-value"}
			tags = []string{"some", "tags"}

			inStep = new(fakes.FakeStep)
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
			step = factory.Put(
				lagertest.NewTestLogger("test"),
				stepMetadata,
				identifier,
				workerMetadata,
				putDelegate,
				resourceConfig,
				tags,
				params,
				resourceTypes,
			).Using(inStep, repo)

			process = ifrit.Invoke(step)
		})

		Context("when repo contains sources", func() {
			var (
				fakeSource        *fakes.FakeArtifactSource
				fakeOtherSource   *fakes.FakeArtifactSource
				fakeMountedSource *fakes.FakeArtifactSource
			)

			BeforeEach(func() {
				fakeSource = new(fakes.FakeArtifactSource)
				fakeOtherSource = new(fakes.FakeArtifactSource)
				fakeMountedSource = new(fakes.FakeArtifactSource)

				repo.RegisterSource("some-source", fakeSource)
				repo.RegisterSource("some-other-source", fakeOtherSource)
				repo.RegisterSource("some-mounted-source", fakeMountedSource)
			})

			Context("when the tracker can initialize the resource", func() {
				var (
					fakeResource        *rfakes.FakeResource
					fakeVersionedSource *rfakes.FakeVersionedSource
				)

				BeforeEach(func() {
					fakeResource = new(rfakes.FakeResource)
					fakeTracker.InitWithSourcesReturns(fakeResource, []string{"some-source", "some-other-source"}, nil)

					fakeVersionedSource = new(rfakes.FakeVersionedSource)
					fakeVersionedSource.VersionReturns(atc.Version{"some": "version"})
					fakeVersionedSource.MetadataReturns([]atc.MetadataField{{"some", "metadata"}})

					fakeResource.PutReturns(fakeVersionedSource)
				})

				It("initializes the resource with the correct type, session, and sources", func() {
					Expect(fakeTracker.InitWithSourcesCallCount()).To(Equal(1))

					_, sm, sid, typ, tags, sources, actualResourceTypes, delegate := fakeTracker.InitWithSourcesArgsForCall(0)
					Expect(sm).To(Equal(stepMetadata))
					Expect(sid).To(Equal(resource.Session{
						ID: worker.Identifier{
							ResourceID: 1234,
							Stage:      db.ContainerStageRun,
						},
						Metadata: worker.Metadata{
							PipelineName:     "some-pipeline",
							Type:             db.ContainerTypePut,
							StepName:         "some-step",
							WorkingDirectory: "/tmp/build/put",
						},
					}))
					Expect(typ).To(Equal(resource.ResourceType("some-resource-type")))
					Expect(tags).To(ConsistOf("some", "tags"))
					Expect(actualResourceTypes).To(Equal(atc.ResourceTypes{
						{
							Name:   "custom-resource",
							Type:   "custom-type",
							Source: atc.Source{"some-custom": "source"},
						},
					}))
					Expect(delegate).To(Equal(putDelegate))

					// TODO: Can we test the map values?
					Expect(sources).To(HaveKey("some-source"))
					Expect(sources).To(HaveKey("some-other-source"))
					Expect(sources).To(HaveKey("some-mounted-source"))
				})

				It("puts the resource with the correct source and params, and the full repository as the artifact source", func() {
					Expect(fakeResource.PutCallCount()).To(Equal(1))

					_, putSource, putParams, putArtifactSource := fakeResource.PutArgsForCall(0)
					Expect(putSource).To(Equal(resourceConfig.Source))
					Expect(putParams).To(Equal(params))

					dest := new(fakes.FakeArtifactDestination)

					err := putArtifactSource.StreamTo(dest)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeSource.StreamToCallCount()).To(Equal(1))

					sourceDest := fakeSource.StreamToArgsForCall(0)
					someStream := new(bytes.Buffer)

					err = sourceDest.StreamIn("foo", someStream)
					Expect(err).NotTo(HaveOccurred())

					Expect(dest.StreamInCallCount()).To(Equal(1))
					destPath, stream := dest.StreamInArgsForCall(0)
					Expect(destPath).To(Equal("some-source/foo"))
					Expect(stream).To(Equal(someStream))

					Expect(fakeOtherSource.StreamToCallCount()).To(Equal(1))

					otherSourceDest := fakeOtherSource.StreamToArgsForCall(0)
					someOtherStream := new(bytes.Buffer)

					err = otherSourceDest.StreamIn("foo", someOtherStream)
					Expect(err).NotTo(HaveOccurred())

					Expect(dest.StreamInCallCount()).To(Equal(2))
					otherDestPath, otherStream := dest.StreamInArgsForCall(1)
					Expect(otherDestPath).To(Equal("some-other-source/foo"))
					Expect(otherStream).To(Equal(someOtherStream))

					Expect(fakeMountedSource.StreamToCallCount()).To(Equal(0))
				})

				It("puts the resource with the io config forwarded", func() {
					Expect(fakeResource.PutCallCount()).To(Equal(1))

					ioConfig, _, _, _ := fakeResource.PutArgsForCall(0)
					Expect(ioConfig.Stdout).To(Equal(stdoutBuf))
					Expect(ioConfig.Stderr).To(Equal(stderrBuf))
				})

				It("runs the get resource action", func() {
					Expect(fakeVersionedSource.RunCallCount()).To(Equal(1))
				})

				It("reports the created version info", func() {
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
					Eventually(putDelegate.CompletedCallCount).Should(Equal(1))

					exitStatus, verionInfo := putDelegate.CompletedArgsForCall(0)
					Expect(exitStatus).To(Equal(ExitStatus(0)))
					Expect(verionInfo).To(Equal(&VersionInfo{
						Version:  atc.Version{"some": "version"},
						Metadata: []atc.MetadataField{{"some", "metadata"}},
					}))
				})

				Context("before initializing the resource", func() {
					var callCountDuringInit chan int

					BeforeEach(func() {
						callCountDuringInit = make(chan int, 1)

						fakeTracker.InitWithSourcesStub = func(lager.Logger, resource.Metadata, resource.Session, resource.ResourceType, atc.Tags, map[string]resource.ArtifactSource, atc.ResourceTypes, worker.ImageFetchingDelegate) (resource.Resource, []string, error) {
							callCountDuringInit <- putDelegate.InitializingCallCount()
							return fakeResource, []string{"some-source", "some-other-source"}, nil
						}
					})

					It("calls the Initializing method on the delegate", func() {
						Expect(<-callCountDuringInit).To(Equal(1))
					})
				})

				Describe("releasing", func() {
					It("releases the resource with the configured containerSuccessTTL", func() {
						<-process.Wait()

						Expect(fakeResource.ReleaseCallCount()).To(BeZero())

						step.Release()
						Expect(fakeResource.ReleaseCallCount()).To(Equal(1))
						Expect(fakeResource.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(worker.FinishedContainerTTL)))
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

				Context("when performing the put fails", func() {
					Context("with an unknown error", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeVersionedSource.RunReturns(disaster)
						})

						It("exits with the failure", func() {
							Eventually(process.Wait()).Should(Receive(Equal(disaster)))
						})

						It("invokes the delegate's Failed callback without completing", func() {
							Eventually(process.Wait()).Should(Receive(Equal(disaster)))

							Expect(putDelegate.CompletedCallCount()).To(BeZero())

							Expect(putDelegate.FailedCallCount()).To(Equal(1))
							Expect(putDelegate.FailedArgsForCall(0)).To(Equal(disaster))
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

					Context("by being interrupted", func() {
						BeforeEach(func() {
							fakeVersionedSource.RunReturns(resource.ErrAborted)
						})

						It("exits with ErrInterrupted", func() {
							Expect(<-process.Wait()).To(Equal(ErrInterrupted))
						})

						It("invokes the delegate's Failed callback without completing", func() {
							<-process.Wait()

							Expect(putDelegate.CompletedCallCount()).To(BeZero())

							Expect(putDelegate.FailedCallCount()).To(Equal(1))
							Expect(putDelegate.FailedArgsForCall(0)).To(Equal(ErrInterrupted))
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

							Expect(putDelegate.FailedCallCount()).To(BeZero())

							Expect(putDelegate.CompletedCallCount()).To(Equal(1))
							status, versionInfo := putDelegate.CompletedArgsForCall(0)
							Expect(status).To(Equal(ExitStatus(1)))
							Expect(versionInfo).To(BeNil())
						})

						It("is not successful", func() {
							Eventually(process.Wait()).Should(Receive(BeNil()))
							Expect(putDelegate.CompletedCallCount()).To(Equal(1))

							var success Success

							Expect(step.Result(&success)).To(BeTrue())
							Expect(bool(success)).To(BeFalse())
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

				Describe("releasing", func() {
					It("releases the resource", func() {
						Expect(fakeResource.ReleaseCallCount()).To(BeZero())

						step.Release()
						Expect(fakeResource.ReleaseCallCount()).To(Equal(1))
					})
				})
			})

			Context("when the tracker fails to initialize the resource", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeTracker.InitWithSourcesReturns(nil, nil, disaster)
				})

				It("exits with the failure", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
				})

				It("invokes the delegate's Failed callback", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))

					Expect(putDelegate.CompletedCallCount()).To(BeZero())

					Expect(putDelegate.FailedCallCount()).To(Equal(1))
					Expect(putDelegate.FailedArgsForCall(0)).To(Equal(disaster))
				})
			})
		})

		Context("when there are no sources in repo", func() {
			var (
				fakeResource        *rfakes.FakeResource
				fakeVersionedSource *rfakes.FakeVersionedSource
			)

			BeforeEach(func() {
				fakeResource = new(rfakes.FakeResource)
				fakeTracker.InitWithSourcesReturns(fakeResource, []string{}, nil)

				fakeVersionedSource = new(rfakes.FakeVersionedSource)
				fakeResource.PutReturns(fakeVersionedSource)
			})

			It("streams in empty source", func() {
				_, _, _, resourceSource := fakeResource.PutArgsForCall(0)
				fakeDestination := new(rfakes.FakeArtifactDestination)
				resourceSource.StreamTo(fakeDestination)

				Expect(fakeDestination.StreamInCallCount()).To(Equal(1))

				path, reader := fakeDestination.StreamInArgsForCall(0)
				Expect(path).To(Equal("."))

				tarReader := tar.NewReader(reader)

				_, err := tarReader.Next()
				Expect(err).To(Equal(io.EOF))
			})
		})
	})
})
