package exec_test

import (
	"archive/tar"
	"errors"
	"io"
	"io/ioutil"
	"os"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/fakes"
	"github.com/concourse/atc/resource"
	rfakes "github.com/concourse/atc/resource/fakes"
	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
)

var identifier = worker.Identifier{
	Name: "some-session-id",
}

var _ = Describe("GardenFactory", func() {
	var (
		fakeTrackerFactory *fakes.FakeTrackerFactory
		fakeTracker        *rfakes.FakeTracker
		fakeWorkerClient   *wfakes.FakeClient

		factory Factory

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		identifier = worker.Identifier{
			Name: "some-session-id",
		}

		stepMetadata testMetadata = []string{"a=1", "b=2"}

		sourceName SourceName = "some-source-name"
	)

	BeforeEach(func() {
		fakeTrackerFactory = new(fakes.FakeTrackerFactory)

		fakeTracker = new(rfakes.FakeTracker)
		fakeTrackerFactory.TrackerForReturns(fakeTracker)

		fakeWorkerClient = new(wfakes.FakeClient)

		factory = NewGardenFactory(fakeWorkerClient, fakeTrackerFactory, func() string { return "" })

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
			step = factory.DependentGet(stepMetadata, sourceName, identifier, getDelegate, resourceConfig, tags, params).Using(inStep, repo)
			process = ifrit.Invoke(step)
		})

		Context("when the tracker can initialize the resource", func() {
			var (
				fakeResource        *rfakes.FakeResource
				fakeVersionedSource *rfakes.FakeVersionedSource
			)

			BeforeEach(func() {
				fakeResource = new(rfakes.FakeResource)
				fakeTracker.InitReturns(fakeResource, nil)

				fakeVersionedSource = new(rfakes.FakeVersionedSource)
				fakeVersionedSource.VersionReturns(atc.Version{"some": "version"})
				fakeVersionedSource.MetadataReturns([]atc.MetadataField{{"some", "metadata"}})

				fakeResource.GetReturns(fakeVersionedSource)
			})

			It("selects a worker satisfying the resource type and tags", func() {
				Ω(fakeWorkerClient.SatisfyingCallCount()).Should(Equal(1))
				Ω(fakeWorkerClient.SatisfyingArgsForCall(0)).Should(Equal(worker.WorkerSpec{
					ResourceType: "some-resource-type",
					Tags:         []string{"some", "tags"},
				}))
			})

			Context("when no workers satisfy the spec", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeWorkerClient.SatisfyingReturns(nil, disaster)
				})

				It("exits with the error", func() {
					Ω(<-process.Wait()).Should(Equal(disaster))
				})
			})

			It("initializes the resource with the correct type and session id, making sure that it is not ephemeral", func() {
				Ω(fakeTracker.InitCallCount()).Should(Equal(1))

				sm, sid, typ, tags, vol := fakeTracker.InitArgsForCall(0)
				Ω(sm).Should(Equal(stepMetadata))
				Ω(sid).Should(Equal(resource.Session{
					ID:        identifier,
					Ephemeral: false,
				}))
				Ω(typ).Should(Equal(resource.ResourceType("some-resource-type")))
				Ω(tags).Should(ConsistOf("some", "tags"))
				Ω(vol).Should(BeZero())
			})

			It("gets the resource with the correct source, params, and version", func() {
				Ω(fakeResource.GetCallCount()).Should(Equal(1))

				_, gotSource, gotParams, gotVersion := fakeResource.GetArgsForCall(0)
				Ω(gotSource).Should(Equal(resourceConfig.Source))
				Ω(gotParams).Should(Equal(params))
				Ω(gotVersion).Should(Equal(version))
			})

			It("gets the resource with the io config forwarded", func() {
				Ω(fakeResource.GetCallCount()).Should(Equal(1))

				ioConfig, _, _, _ := fakeResource.GetArgsForCall(0)
				Ω(ioConfig.Stdout).Should(Equal(stdoutBuf))
				Ω(ioConfig.Stderr).Should(Equal(stderrBuf))
			})

			It("runs the get resource action", func() {
				Ω(fakeVersionedSource.RunCallCount()).Should(Equal(1))
			})

			It("reports the fetched version info", func() {
				var info VersionInfo
				Ω(step.Result(&info)).Should(BeTrue())
				Ω(info.Version).Should(Equal(atc.Version{"some": "version"}))
				Ω(info.Metadata).Should(Equal([]atc.MetadataField{{"some", "metadata"}}))
			})

			It("is successful", func() {
				Eventually(process.Wait()).Should(Receive(BeNil()))

				var success Success
				Ω(step.Result(&success)).Should(BeTrue())
				Ω(bool(success)).Should(BeTrue())
			})

			It("completes via the delegate", func() {
				Eventually(getDelegate.CompletedCallCount).Should(Equal(1))

				exitStatus, versionInfo := getDelegate.CompletedArgsForCall(0)
				Ω(exitStatus).Should(Equal(ExitStatus(0)))
				Ω(versionInfo).Should(Equal(&VersionInfo{
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

				It("invokes the delegate's Failed callback without completing", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))

					Ω(getDelegate.CompletedCallCount()).Should(BeZero())

					Ω(getDelegate.FailedCallCount()).Should(Equal(1))
					Ω(getDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
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

						Ω(getDelegate.FailedCallCount()).Should(BeZero())

						Ω(getDelegate.CompletedCallCount()).Should(Equal(1))

						status, versionInfo := getDelegate.CompletedArgsForCall(0)
						Ω(status).Should(Equal(ExitStatus(1)))
						Ω(versionInfo).Should(BeNil())
					})

					It("is not successful", func() {
						Eventually(process.Wait()).Should(Receive(BeNil()))
						Ω(getDelegate.CompletedCallCount()).Should(Equal(1))

						var success Success

						Ω(step.Result(&success)).Should(BeTrue())
						Ω(bool(success)).Should(BeFalse())
					})
				})
			})

			Describe("releasing", func() {
				It("releases the resource", func() {
					Ω(fakeResource.ReleaseCallCount()).Should(BeZero())

					step.Release()
					Ω(fakeResource.ReleaseCallCount()).Should(Equal(1))
				})
			})

			Describe("the source registered with the repository", func() {
				var artifactSource ArtifactSource

				JustBeforeEach(func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))

					var found bool
					artifactSource, found = repo.SourceFor(sourceName)
					Ω(found).Should(BeTrue())
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
							Ω(err).ShouldNot(HaveOccurred())

							Ω(fakeVersionedSource.StreamOutCallCount()).Should(Equal(1))
							Ω(fakeVersionedSource.StreamOutArgsForCall(0)).Should(Equal("."))

							Ω(fakeDestination.StreamInCallCount()).Should(Equal(1))
							dest, src := fakeDestination.StreamInArgsForCall(0)
							Ω(dest).Should(Equal("."))
							Ω(src).Should(Equal(streamedOut))
						})

						Context("when streaming out of the versioned source fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeVersionedSource.StreamOutReturns(nil, disaster)
							})

							It("returns the error", func() {
								Ω(artifactSource.StreamTo(fakeDestination)).Should(Equal(disaster))
							})
						})

						Context("when streaming in to the destination fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeDestination.StreamInReturns(disaster)
							})

							It("returns the error", func() {
								Ω(artifactSource.StreamTo(fakeDestination)).Should(Equal(disaster))
							})
						})
					})

					Context("when the resource cannot stream out", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeVersionedSource.StreamOutReturns(nil, disaster)
						})

						It("returns the error", func() {
							Ω(artifactSource.StreamTo(fakeDestination)).Should(Equal(disaster))
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
								Ω(err).ShouldNot(HaveOccurred())

								_, err = tarWriter.Write([]byte(fileContent))
								Ω(err).ShouldNot(HaveOccurred())
							})

							It("streams out the given path", func() {
								reader, err := artifactSource.StreamFile("some-path")
								Ω(err).ShouldNot(HaveOccurred())

								Ω(ioutil.ReadAll(reader)).Should(Equal([]byte(fileContent)))

								Ω(fakeVersionedSource.StreamOutArgsForCall(0)).Should(Equal("some-path"))
							})

							Describe("closing the stream", func() {
								It("closes the stream from the versioned source", func() {
									reader, err := artifactSource.StreamFile("some-path")
									Ω(err).ShouldNot(HaveOccurred())

									Ω(tarBuffer.Closed()).Should(BeFalse())

									err = reader.Close()
									Ω(err).ShouldNot(HaveOccurred())

									Ω(tarBuffer.Closed()).Should(BeTrue())
								})
							})
						})

						Context("but the stream is empty", func() {
							It("returns ErrFileNotFound", func() {
								_, err := artifactSource.StreamFile("some-path")
								Ω(err).Should(MatchError(FileNotFoundError{Path: "some-path"}))
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
							Ω(err).Should(Equal(disaster))
						})
					})
				})
			})
		})

		Context("when the tracker fails to initialize the resource", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeTracker.InitReturns(nil, disaster)
			})

			It("exits with the failure", func() {
				Eventually(process.Wait()).Should(Receive(Equal(disaster)))
			})

			It("invokes the delegate's Failed callback", func() {
				Eventually(process.Wait()).Should(Receive(Equal(disaster)))

				Ω(getDelegate.CompletedCallCount()).Should(BeZero())

				Ω(getDelegate.FailedCallCount()).Should(Equal(1))
				Ω(getDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
			})
		})
	})
})
