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
	"github.com/concourse/baggageclaim"
	bfakes "github.com/concourse/baggageclaim/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
)

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

	Describe("Get", func() {
		var (
			getDelegate    *fakes.FakeGetDelegate
			resourceConfig atc.ResourceConfig
			params         atc.Params
			version        atc.Version
			tags           []string

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
		})

		JustBeforeEach(func() {
			step = factory.Get(
				lagertest.NewTestLogger("test"),
				stepMetadata,
				sourceName,
				identifier,
				getDelegate,
				resourceConfig,
				params,
				tags,
				version,
			).Using(inStep, repo)

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

			Context("when the worker supports volumes", func() {
				var fakeBaggageclaimClient *bfakes.FakeClient

				BeforeEach(func() {
					fakeBaggageclaimClient = new(bfakes.FakeClient)
					satisfiedWorker.VolumeManagerReturns(fakeBaggageclaimClient, true)
				})

				Context("when a volume for the resource is already present", func() {
					var foundVolume *bfakes.FakeVolume

					BeforeEach(func() {
						foundVolume = new(bfakes.FakeVolume)
						foundVolume.HandleReturns("found-volume-handle")
						fakeBaggageclaimClient.ListVolumesReturns([]baggageclaim.Volume{foundVolume}, nil)
					})

					It("looked up the volume with the correct properties", func() {
						_, spec := fakeBaggageclaimClient.ListVolumesArgsForCall(0)
						Ω(spec).Should(Equal(baggageclaim.VolumeProperties{
							"resource-type":    "some-resource-type",
							"resource-version": `{"some-version":"some-value"}`,
							"resource-source":  "968e27f71617a029e58a09fb53895f1e1875b51bdaa11293ddc2cb335960875cb42c19ae8bc696caec88d55221f33c2bcc3278a7d15e8d13f23782d1a05564f1",
							"resource-params":  "7cee8d669e89dee0c318bd9d2788c513bab8a900322ae593247fedd95bffa23b5be71f54326dffd9c2e65e13ca995fca9037d162232b9264a394e8d65ce8de79",
							"initialized":      "yep",
						}))
					})

					It("initializes the tracker with the chosen worker", func() {
						Ω(fakeTrackerFactory.TrackerForCallCount()).Should(Equal(1))
						Ω(fakeTrackerFactory.TrackerForArgsForCall(0)).Should(Equal(satisfiedWorker))
					})

					It("initializes the resource with the volume", func() {
						Ω(fakeTracker.InitCallCount()).Should(Equal(1))
						_, sm, sid, typ, tags, vol := fakeTracker.InitArgsForCall(0)
						Ω(sm).Should(Equal(stepMetadata))
						Ω(sid).Should(Equal(resource.Session{
							ID:        identifier,
							Ephemeral: false,
						}))
						Ω(typ).Should(Equal(resource.ResourceType("some-resource-type")))
						Ω(tags).Should(ConsistOf("some", "tags"))
						Ω(vol).Should(Equal(resource.VolumeMount{
							Volume:    foundVolume,
							MountPath: "/tmp/build/get",
						}))
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

					It("does not run the get resource action", func() {
						Ω(fakeVersionedSource.RunCallCount()).Should(Equal(0))
					})

					It("logs a helpful message", func() {
						Ω(stdoutBuf).Should(gbytes.Say("using version of resource found in cache\n"))
					})

					Context("when the resource has a cached volume", func() {
						var mountedVolume *bfakes.FakeVolume

						BeforeEach(func() {
							mountedVolume = new(bfakes.FakeVolume)
							fakeResource.CacheVolumeReturns(mountedVolume, true, nil)
						})

						Context("when it differs from the created volume", func() {
							BeforeEach(func() {
								mountedVolume.HandleReturns("some-other-handle")
							})

							It("stops heartbeating to the created volume", func() {
								Ω(foundVolume.ReleaseCallCount()).Should(Equal(1))
							})
						})

						Context("when it is the same as the created volume", func() {
							BeforeEach(func() {
								mountedVolume.HandleReturns("found-volume-handle")
							})

							It("stops heartbeating as the container heartbeats for us", func() {
								Ω(foundVolume.ReleaseCallCount()).Should(Equal(1))
							})
						})
					})
				})

				Context("when multiple volumes for the resource are already present", func() {
					var aVolume *bfakes.FakeVolume
					var bVolume *bfakes.FakeVolume

					BeforeEach(func() {
						aVolume = new(bfakes.FakeVolume)
						aVolume.HandleReturns("a")
						bVolume = new(bfakes.FakeVolume)
						bVolume.HandleReturns("b")
					})

					Context("with a, b order", func() {
						BeforeEach(func() {
							fakeBaggageclaimClient.ListVolumesReturns([]baggageclaim.Volume{aVolume, bVolume}, nil)
						})

						It("selects the volume based on the lowest alphabetical name", func() {
							Ω(aVolume.SetTTLCallCount()).Should(Equal(0))
							Ω(bVolume.ReleaseCallCount()).Should(Equal(1))
							Ω(bVolume.SetTTLCallCount()).Should(Equal(1))
							Ω(bVolume.SetTTLArgsForCall(0)).Should(Equal(uint(60)))
						})
					})

					Context("with b, a order", func() {
						BeforeEach(func() {
							fakeBaggageclaimClient.ListVolumesReturns([]baggageclaim.Volume{bVolume, aVolume}, nil)
						})

						It("selects the volume based on the lowest alphabetical name", func() {
							Ω(aVolume.SetTTLCallCount()).Should(Equal(0))
							Ω(bVolume.ReleaseCallCount()).Should(Equal(1))
							Ω(bVolume.SetTTLCallCount()).Should(Equal(1))
							Ω(bVolume.SetTTLArgsForCall(0)).Should(Equal(uint(60)))
						})
					})
				})

				Context("when a volume for the resource is not already present", func() {
					var createdVolume *bfakes.FakeVolume

					BeforeEach(func() {
						createdVolume = new(bfakes.FakeVolume)
						createdVolume.HandleReturns("created-volume-handle")

						fakeBaggageclaimClient.CreateVolumeReturns(createdVolume, nil)
					})

					It("created the volume with the correct properties (notably, without 'initialized')", func() {
						_, spec := fakeBaggageclaimClient.CreateVolumeArgsForCall(0)
						Ω(spec).Should(Equal(baggageclaim.VolumeSpec{
							Properties: baggageclaim.VolumeProperties{
								"resource-type":    "some-resource-type",
								"resource-version": `{"some-version":"some-value"}`,
								"resource-source":  "968e27f71617a029e58a09fb53895f1e1875b51bdaa11293ddc2cb335960875cb42c19ae8bc696caec88d55221f33c2bcc3278a7d15e8d13f23782d1a05564f1",
								"resource-params":  "7cee8d669e89dee0c318bd9d2788c513bab8a900322ae593247fedd95bffa23b5be71f54326dffd9c2e65e13ca995fca9037d162232b9264a394e8d65ce8de79",
							},
							TTLInSeconds: 60 * 60 * 24,
							Privileged:   true,
						}))
					})

					It("initializes the resource with the volume", func() {
						Ω(fakeTracker.InitCallCount()).Should(Equal(1))
						_, sm, sid, typ, tags, vol := fakeTracker.InitArgsForCall(0)
						Ω(sm).Should(Equal(stepMetadata))
						Ω(sid).Should(Equal(resource.Session{
							ID:        identifier,
							Ephemeral: false,
						}))
						Ω(typ).Should(Equal(resource.ResourceType("some-resource-type")))
						Ω(tags).Should(ConsistOf("some", "tags"))
						Ω(vol).Should(Equal(resource.VolumeMount{
							Volume:    createdVolume,
							MountPath: "/tmp/build/get",
						}))
					})

					Context("when the resource has a cached volume", func() {
						var mountedVolume *bfakes.FakeVolume

						BeforeEach(func() {
							mountedVolume = new(bfakes.FakeVolume)
							fakeResource.CacheVolumeReturns(mountedVolume, true, nil)
						})

						Context("when it differs from the created volume", func() {
							BeforeEach(func() {
								mountedVolume.HandleReturns("some-other-handle")
							})

							It("stops heartbeating to the created volume", func() {
								Ω(createdVolume.ReleaseCallCount()).Should(Equal(1))
							})
						})

						Context("when it is the same as the created volume", func() {
							BeforeEach(func() {
								mountedVolume.HandleReturns("created-volume-handle")
							})

							It("stops heartbeating as the container heartbeats for us", func() {
								Ω(createdVolume.ReleaseCallCount()).Should(Equal(1))
							})
						})
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

					Context("after the 'get' action completes", func() {
						BeforeEach(func() {
							fakeVersionedSource.RunReturns(nil)
						})

						Context("when the resource has a cached volume", func() {
							var mountedVolume *bfakes.FakeVolume

							BeforeEach(func() {
								mountedVolume = new(bfakes.FakeVolume)
								fakeResource.CacheVolumeReturns(mountedVolume, true, nil)
							})

							It("marks the volume as initialized after the 'get' action completes", func() {
								Ω(mountedVolume.SetPropertyCallCount()).Should(Equal(1))
								name, value := mountedVolume.SetPropertyArgsForCall(0)
								Ω(name).Should(Equal("initialized"))
								Ω(value).Should(Equal("yep"))
							})

							It("does NOT mark the created volume as initialized, as it may be different (e.g. ATC crash)", func() {
								Ω(createdVolume.SetPropertyCallCount()).Should(Equal(0))
							})
						})

						Context("when the resource does not have a cached volume (to deal with upgrade path)", func() {
							BeforeEach(func() {
								fakeResource.CacheVolumeReturns(nil, false, nil)
							})

							It("does not mark the volume as initialized", func() {
								Ω(createdVolume.SetPropertyCallCount()).Should(Equal(0))
							})
						})

						Context("when getting the resource's cached volume fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeResource.CacheVolumeReturns(nil, false, disaster)
							})

							It("returns the error", func() {
								Ω(<-process.Wait()).Should(Equal(disaster))
							})
						})
					})

					Context("after the 'get' action fails", func() {
						BeforeEach(func() {
							fakeVersionedSource.RunReturns(errors.New("nope"))
						})

						It("does not mark the volume as initialized", func() {
							Ω(createdVolume.SetPropertyCallCount()).Should(Equal(0))
						})
					})
				})
			})

			Context("when the worker does not support volumes", func() {
				BeforeEach(func() {
					satisfiedWorker.VolumeManagerReturns(nil, false)
				})

				It("initializes the resource with the correct type and session id, making sure that it is not ephemeral, and with no volume", func() {
					Ω(fakeTracker.InitCallCount()).Should(Equal(1))

					_, sm, sid, typ, tags, vol := fakeTracker.InitArgsForCall(0)
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
							Eventually(process.Wait()).Should(Receive(BeNil()))

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
			})

			It("reports the fetched version info", func() {
				var info VersionInfo
				Ω(step.Result(&info)).Should(BeTrue())
				Ω(info.Version).Should(Equal(atc.Version{"some": "version"}))
				Ω(info.Metadata).Should(Equal([]atc.MetadataField{{"some", "metadata"}}))
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

			It("is successful", func() {
				Eventually(process.Wait()).Should(Receive(BeNil()))

				var success Success
				Ω(step.Result(&success)).Should(BeTrue())
				Ω(bool(success)).Should(BeTrue())
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

				Describe("getting a volume from a worker", func() {
					var fakeWorker *wfakes.FakeWorker

					BeforeEach(func() {
						fakeWorker = new(wfakes.FakeWorker)
					})

					Context("when the worker has a volume manager", func() {
						var fakeBaggageclaimClient *bfakes.FakeClient

						BeforeEach(func() {
							fakeBaggageclaimClient = new(bfakes.FakeClient)
							fakeWorker.VolumeManagerReturns(fakeBaggageclaimClient, true)
						})

						Context("when the worker has the volume", func() {
							var foundVolume *bfakes.FakeVolume

							BeforeEach(func() {
								foundVolume = new(bfakes.FakeVolume)
								foundVolume.HandleReturns("found-volume-handle")
								fakeBaggageclaimClient.ListVolumesReturns([]baggageclaim.Volume{foundVolume}, nil)
							})

							It("returns the volume and true", func() {
								volume, found, err := artifactSource.VolumeOn(fakeWorker)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(found).Should(BeTrue())
								Ω(volume).Should(Equal(foundVolume))

								_, props := fakeBaggageclaimClient.ListVolumesArgsForCall(0)
								Ω(props).Should(Equal(baggageclaim.VolumeProperties{
									"resource-type":    "some-resource-type",
									"resource-version": `{"some-version":"some-value"}`,
									"resource-source":  "968e27f71617a029e58a09fb53895f1e1875b51bdaa11293ddc2cb335960875cb42c19ae8bc696caec88d55221f33c2bcc3278a7d15e8d13f23782d1a05564f1",
									"resource-params":  "7cee8d669e89dee0c318bd9d2788c513bab8a900322ae593247fedd95bffa23b5be71f54326dffd9c2e65e13ca995fca9037d162232b9264a394e8d65ce8de79",
									"initialized":      "yep",
								}))
							})
						})

						Context("when the worker has multiple volumes", func() {
							var aVolume *bfakes.FakeVolume
							var bVolume *bfakes.FakeVolume

							BeforeEach(func() {
								aVolume = new(bfakes.FakeVolume)
								aVolume.HandleReturns("a")
								bVolume = new(bfakes.FakeVolume)
								bVolume.HandleReturns("b")
							})

							Context("with a, b order", func() {
								BeforeEach(func() {
									fakeBaggageclaimClient.ListVolumesReturns([]baggageclaim.Volume{aVolume, bVolume}, nil)
								})

								It("selects the volume based on the lowest alphabetical name", func() {
									volume, found, err := artifactSource.VolumeOn(fakeWorker)
									Ω(err).ShouldNot(HaveOccurred())
									Ω(found).Should(BeTrue())
									Ω(volume).Should(Equal(aVolume))
								})
							})

							Context("with b, a order", func() {
								BeforeEach(func() {
									fakeBaggageclaimClient.ListVolumesReturns([]baggageclaim.Volume{bVolume, aVolume}, nil)
								})

								It("selects the volume based on the lowest alphabetical name", func() {
									volume, found, err := artifactSource.VolumeOn(fakeWorker)
									Ω(err).ShouldNot(HaveOccurred())
									Ω(found).Should(BeTrue())
									Ω(volume).Should(Equal(aVolume))
								})
							})
						})

						Context("when the worker does not have the volume", func() {
							BeforeEach(func() {
								fakeBaggageclaimClient.ListVolumesReturns([]baggageclaim.Volume{}, nil)
							})

							It("returns false", func() {
								_, found, err := artifactSource.VolumeOn(fakeWorker)
								Ω(err).ShouldNot(HaveOccurred())
								Ω(found).Should(BeFalse())
							})
						})

						Context("when looking up the volume fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeBaggageclaimClient.ListVolumesReturns(nil, disaster)
							})

							It("returns the error", func() {
								_, _, err := artifactSource.VolumeOn(fakeWorker)
								Ω(err).Should(Equal(disaster))
							})
						})
					})

					Context("when the worker does not have a volume manager", func() {
						BeforeEach(func() {
							fakeWorker.VolumeManagerReturns(nil, false)
						})

						It("returns false", func() {
							_, found, err := artifactSource.VolumeOn(fakeWorker)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(found).Should(BeFalse())
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
