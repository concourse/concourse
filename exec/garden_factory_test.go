package exec_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"

	"github.com/cloudfoundry-incubator/garden"
	gfakes "github.com/cloudfoundry-incubator/garden/fakes"
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

const sessionID = "some-session-id"

var _ = Describe("GardenFactory", func() {
	var (
		fakeTracker      *rfakes.FakeTracker
		fakeWorkerClient *wfakes.FakeClient

		factory Factory

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer
	)

	BeforeEach(func() {
		fakeTracker = new(rfakes.FakeTracker)
		fakeWorkerClient = new(wfakes.FakeClient)

		factory = NewGardenFactory(fakeWorkerClient, fakeTracker)

		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
	})

	Describe("Hijack", func() {
		var (
			processSpec atc.HijackProcessSpec
			processIO   IOConfig

			hijackedProcess HijackedProcess
			hijackErr       error
		)

		BeforeEach(func() {
			processSpec = atc.HijackProcessSpec{
				Path: "ls",
			}

			processIO = IOConfig{
				Stdin:  new(bytes.Buffer),
				Stdout: new(bytes.Buffer),
				Stderr: new(bytes.Buffer),
			}
		})

		JustBeforeEach(func() {
			hijackedProcess, hijackErr = factory.Hijack(sessionID, processIO, processSpec)
		})

		Context("when finding the session's container succeeds", func() {
			var fakeContainer *wfakes.FakeContainer

			BeforeEach(func() {
				fakeContainer = new(wfakes.FakeContainer)
				fakeWorkerClient.LookupReturns(fakeContainer, nil)
			})

			Context("and the container can spawn processes", func() {
				var fakeProcess *gfakes.FakeProcess

				BeforeEach(func() {
					fakeProcess = new(gfakes.FakeProcess)
					fakeContainer.RunReturns(fakeProcess, nil)
				})

				It("looks up by session id", func() {
					Ω(fakeWorkerClient.LookupArgsForCall(0)).Should(Equal(sessionID))
				})

				It("succeeds", func() {
					Ω(hijackErr).ShouldNot(HaveOccurred())
				})

				It("releases the container", func() {
					Ω(fakeContainer.ReleaseCallCount()).Should(Equal(1))
				})

				It("spawns the process with the given spec and IO config", func() {
					Ω(fakeContainer.RunCallCount()).Should(Equal(1))

					ranSpec, ranIO := fakeContainer.RunArgsForCall(0)
					Ω(ranSpec).Should(Equal(garden.ProcessSpec{
						Path: "ls",
					}))
					Ω(ranIO).Should(Equal(garden.ProcessIO(processIO)))
				})

				Describe("waiting on the process", func() {
					BeforeEach(func() {
						fakeProcess.WaitReturns(123, nil)
					})

					It("delegates to the garden process", func() {
						Ω(hijackedProcess.Wait()).Should(Equal(123))
					})
				})

				Describe("updating the process's TTY", func() {
					It("delegates to the garden process", func() {
						err := hijackedProcess.SetTTY(atc.HijackTTYSpec{
							WindowSize: atc.HijackWindowSize{
								Columns: 123,
								Rows:    456,
							},
						})
						Ω(err).ShouldNot(HaveOccurred())

						Ω(fakeProcess.SetTTYCallCount()).Should(Equal(1))
						Ω(fakeProcess.SetTTYArgsForCall(0)).Should(Equal(garden.TTYSpec{
							WindowSize: &garden.WindowSize{
								Columns: 123,
								Rows:    456,
							},
						}))
					})
				})
			})

			Context("when the container cannot spawn processes", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeContainer.RunReturns(nil, disaster)
				})

				It("returns the error", func() {
					Ω(hijackErr).Should(Equal(disaster))
				})
			})
		})

		Context("when finding the session's container fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeWorkerClient.LookupReturns(nil, disaster)
			})

			It("returns the error", func() {
				Ω(hijackErr).Should(Equal(disaster))
			})
		})
	})

	Describe("Get", func() {
		var (
			getDelegate    *fakes.FakeGetDelegate
			resourceConfig atc.ResourceConfig
			params         atc.Params
			version        atc.Version

			inSource ArtifactSource

			source  ArtifactSource
			process ifrit.Process
		)

		BeforeEach(func() {
			getDelegate = new(fakes.FakeGetDelegate)
			getDelegate.StdoutReturns(stdoutBuf)
			getDelegate.StderrReturns(stderrBuf)

			resourceConfig = atc.ResourceConfig{
				Name:   "some-resource",
				Type:   "some-resource-type",
				Source: atc.Source{"some": "source"},
			}

			params = atc.Params{"some-param": "some-value"}

			version = atc.Version{"some-version": "some-value"}

			inSource = nil // not needed for Get
		})

		JustBeforeEach(func() {
			source = factory.Get(sessionID, getDelegate, resourceConfig, params, version).Using(inSource)
			process = ifrit.Invoke(source)
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

			It("initializes the resource with the correct type and session id", func() {
				Ω(fakeTracker.InitCallCount()).Should(Equal(1))

				sid, typ := fakeTracker.InitArgsForCall(0)
				Ω(sid).Should(Equal(resource.SessionID(sessionID)))
				Ω(typ).Should(Equal(resource.ResourceType("some-resource-type")))
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

			It("executes the get resource action", func() {
				Ω(fakeVersionedSource.RunCallCount()).Should(Equal(1))
			})

			It("reports the fetched version info", func() {
				var info VersionInfo
				Ω(source.Result(&info)).Should(BeTrue())
				Ω(info.Version).Should(Equal(atc.Version{"some": "version"}))
				Ω(info.Metadata).Should(Equal([]atc.MetadataField{{"some", "metadata"}}))
			})

			It("completes via the delegate", func() {
				Eventually(getDelegate.CompletedCallCount).Should(Equal(1))

				Ω(getDelegate.CompletedArgsForCall(0)).Should(Equal(VersionInfo{
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
			})

			Describe("releasing", func() {
				Context("when destroying the resource succeeds", func() {
					BeforeEach(func() {
						fakeResource.DestroyReturns(nil)
					})

					It("destroys the resource", func() {
						Ω(fakeResource.ReleaseCallCount()).Should(BeZero())

						err := source.Release()
						Ω(err).ShouldNot(HaveOccurred())

						Ω(fakeResource.DestroyCallCount()).Should(Equal(1))
					})
				})

				Context("when destroying the resource fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeResource.DestroyReturns(disaster)
					})

					It("returns the error", func() {
						err := source.Release()
						Ω(err).Should(Equal(disaster))
					})
				})
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
						err := source.StreamTo(fakeDestination)
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
							Ω(source.StreamTo(fakeDestination)).Should(Equal(disaster))
						})
					})

					Context("when streaming in to the destination fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeDestination.StreamInReturns(disaster)
						})

						It("returns the error", func() {
							Ω(source.StreamTo(fakeDestination)).Should(Equal(disaster))
						})
					})
				})

				Context("when the resource cannot stream out", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeVersionedSource.StreamOutReturns(nil, disaster)
					})

					It("returns the error", func() {
						Ω(source.StreamTo(fakeDestination)).Should(Equal(disaster))
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
							reader, err := source.StreamFile("some-path")
							Ω(err).ShouldNot(HaveOccurred())

							Ω(ioutil.ReadAll(reader)).Should(Equal([]byte(fileContent)))

							Ω(fakeVersionedSource.StreamOutArgsForCall(0)).Should(Equal("some-path"))
						})

						Describe("closing the stream", func() {
							It("closes the stream from the versioned source", func() {
								reader, err := source.StreamFile("some-path")
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
							_, err := source.StreamFile("some-path")
							Ω(err).Should(Equal(ErrFileNotFound))
						})
					})
				})

				Context("when the resource cannot stream out", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeVersionedSource.StreamOutReturns(nil, disaster)
					})

					It("returns the error", func() {
						_, err := source.StreamFile("some-path")
						Ω(err).Should(Equal(disaster))
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

			Describe("releasing", func() {
				JustBeforeEach(func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
				})

				It("succeeds", func() {
					err := source.Release()
					Ω(err).ShouldNot(HaveOccurred())
				})
			})
		})
	})

	Describe("Put", func() {
		var (
			putDelegate    *fakes.FakePutDelegate
			resourceConfig atc.ResourceConfig
			params         atc.Params

			inSource *fakes.FakeArtifactSource

			source  ArtifactSource
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

			inSource = new(fakes.FakeArtifactSource)
		})

		JustBeforeEach(func() {
			source = factory.Put(sessionID, putDelegate, resourceConfig, params).Using(inSource)
			process = ifrit.Invoke(source)
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

				fakeResource.PutReturns(fakeVersionedSource)
			})

			It("initializes the resource with the correct type and session id", func() {
				Ω(fakeTracker.InitCallCount()).Should(Equal(1))

				sid, typ := fakeTracker.InitArgsForCall(0)
				Ω(sid).Should(Equal(resource.SessionID(sessionID)))
				Ω(typ).Should(Equal(resource.ResourceType("some-resource-type")))
			})

			It("puts the resource with the correct source and params, and the given source as the artifact source", func() {
				Ω(fakeResource.PutCallCount()).Should(Equal(1))

				_, putSource, putParams, putArtifactSource := fakeResource.PutArgsForCall(0)
				Ω(putSource).Should(Equal(resourceConfig.Source))
				Ω(putParams).Should(Equal(params))

				dest := new(fakes.FakeArtifactDestination)

				err := putArtifactSource.StreamTo(dest)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(inSource.StreamToCallCount()).Should(Equal(1))
				Ω(inSource.StreamToArgsForCall(0)).Should(Equal(dest))
			})

			It("puts the resource with the io config forwarded", func() {
				Ω(fakeResource.PutCallCount()).Should(Equal(1))

				ioConfig, _, _, _ := fakeResource.PutArgsForCall(0)
				Ω(ioConfig.Stdout).Should(Equal(stdoutBuf))
				Ω(ioConfig.Stderr).Should(Equal(stderrBuf))
			})

			It("executes the get resource action", func() {
				Ω(fakeVersionedSource.RunCallCount()).Should(Equal(1))
			})

			It("reports the created version info", func() {
				var info VersionInfo
				Ω(source.Result(&info)).Should(BeTrue())
				Ω(info.Version).Should(Equal(atc.Version{"some": "version"}))
				Ω(info.Metadata).Should(Equal([]atc.MetadataField{{"some", "metadata"}}))
			})

			It("completes via the delegate", func() {
				Eventually(putDelegate.CompletedCallCount).Should(Equal(1))

				Ω(putDelegate.CompletedArgsForCall(0)).Should(Equal(VersionInfo{
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

					Ω(putDelegate.CompletedCallCount()).Should(BeZero())

					Ω(putDelegate.FailedCallCount()).Should(Equal(1))
					Ω(putDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
				})
			})

			Describe("releasing", func() {
				Context("when destroying the resource succeeds", func() {
					BeforeEach(func() {
						fakeResource.DestroyReturns(nil)
					})

					It("destroys the resource", func() {
						Ω(fakeResource.ReleaseCallCount()).Should(BeZero())

						err := source.Release()
						Ω(err).ShouldNot(HaveOccurred())

						Ω(fakeResource.DestroyCallCount()).Should(Equal(1))
					})
				})

				Context("when destroying the resource fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeResource.DestroyReturns(disaster)
					})

					It("returns the error", func() {
						err := source.Release()
						Ω(err).Should(Equal(disaster))
					})
				})
			})

			Describe("streaming to a destination", func() {
				var fakeDestination *fakes.FakeArtifactDestination

				BeforeEach(func() {
					fakeDestination = new(fakes.FakeArtifactDestination)
				})

				Context("when the resource can stream out", func() {
					var streamedOut io.ReadCloser

					BeforeEach(func() {
						streamedOut = gbytes.NewBuffer()
						fakeVersionedSource.StreamOutReturns(streamedOut, nil)
					})

					It("streams the resource to the destination", func() {
						err := source.StreamTo(fakeDestination)
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
							Ω(source.StreamTo(fakeDestination)).Should(Equal(disaster))
						})
					})

					Context("when streaming in to the destination fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeDestination.StreamInReturns(disaster)
						})

						It("returns the error", func() {
							Ω(source.StreamTo(fakeDestination)).Should(Equal(disaster))
						})
					})
				})

				Context("when the resource cannot stream out", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeVersionedSource.StreamOutReturns(nil, disaster)
					})

					It("returns the error", func() {
						Ω(source.StreamTo(fakeDestination)).Should(Equal(disaster))
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
							reader, err := source.StreamFile("some-path")
							Ω(err).ShouldNot(HaveOccurred())

							Ω(ioutil.ReadAll(reader)).Should(Equal([]byte(fileContent)))

							Ω(fakeVersionedSource.StreamOutArgsForCall(0)).Should(Equal("some-path"))
						})

						Describe("closing the stream", func() {
							It("closes the stream from the versioned source", func() {
								reader, err := source.StreamFile("some-path")
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
							_, err := source.StreamFile("some-path")
							Ω(err).Should(Equal(ErrFileNotFound))
						})
					})
				})

				Context("when the resource cannot stream out", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeVersionedSource.StreamOutReturns(nil, disaster)
					})

					It("returns the error", func() {
						_, err := source.StreamFile("some-path")
						Ω(err).Should(Equal(disaster))
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

				Ω(putDelegate.CompletedCallCount()).Should(BeZero())

				Ω(putDelegate.FailedCallCount()).Should(Equal(1))
				Ω(putDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
			})

			Describe("releasing", func() {
				JustBeforeEach(func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
				})

				It("succeeds", func() {
					err := source.Release()
					Ω(err).ShouldNot(HaveOccurred())
				})
			})
		})
	})

	Describe("Execute", func() {
		var (
			executeDelegate *fakes.FakeExecuteDelegate
			privileged      Privileged
			configSource    *fakes.FakeBuildConfigSource

			inSource *fakes.FakeArtifactSource

			source  ArtifactSource
			process ifrit.Process
		)

		BeforeEach(func() {
			executeDelegate = new(fakes.FakeExecuteDelegate)
			executeDelegate.StdoutReturns(stdoutBuf)
			executeDelegate.StderrReturns(stderrBuf)

			privileged = false
			configSource = new(fakes.FakeBuildConfigSource)

			inSource = new(fakes.FakeArtifactSource)
		})

		JustBeforeEach(func() {
			source = factory.Execute(sessionID, executeDelegate, privileged, configSource).Using(inSource)
			process = ifrit.Invoke(source)
		})

		Context("when the container does not yet exist", func() {
			BeforeEach(func() {
				fakeWorkerClient.LookupReturns(nil, errors.New("nope"))
			})

			Context("when the getting the config works", func() {
				var fetchedConfig atc.BuildConfig

				BeforeEach(func() {
					fetchedConfig = atc.BuildConfig{
						Platform: "some-platform",
						Tags:     []string{"some", "tags"},
						Image:    "some-image",
						Params:   map[string]string{"SOME": "params"},
						Run: atc.BuildRunConfig{
							Path: "ls",
							Args: []string{"some", "args"},
						},
					}

					configSource.FetchConfigReturns(fetchedConfig, nil)

					inSource.StreamToReturns(nil)
				})

				Context("when creating the build's container works", func() {
					var (
						fakeContainer *wfakes.FakeContainer
						fakeProcess   *gfakes.FakeProcess
					)

					BeforeEach(func() {
						fakeContainer = new(wfakes.FakeContainer)
						fakeContainer.HandleReturns("some-handle")
						fakeWorkerClient.CreateContainerReturns(fakeContainer, nil)

						fakeProcess = new(gfakes.FakeProcess)
						fakeProcess.IDReturns(42)
						fakeContainer.RunReturns(fakeProcess, nil)

						fakeContainer.StreamInReturns(nil)
					})

					Describe("before having created the container", func() {
						BeforeEach(func() {
							executeDelegate.InitializingStub = func(atc.BuildConfig) {
								defer GinkgoRecover()
								Ω(fakeWorkerClient.CreateContainerCallCount()).Should(BeZero())
							}
						})

						It("invokes the delegate's Initializing callback", func() {
							Ω(executeDelegate.InitializingCallCount()).Should(Equal(1))
							Ω(executeDelegate.InitializingArgsForCall(0)).Should(Equal(fetchedConfig))
						})
					})

					It("looked up the container via the session ID", func() {
						Ω(fakeWorkerClient.LookupArgsForCall(0)).Should(Equal(sessionID))
					})

					It("gets the config from the input artifact soruce", func() {
						Ω(configSource.FetchConfigCallCount()).Should(Equal(1))

						source := configSource.FetchConfigArgsForCall(0)
						Ω(source).Should(Equal(inSource))
					})

					It("creates a container with the config's image and the session ID as the handle", func() {
						Ω(fakeWorkerClient.CreateContainerCallCount()).Should(Equal(1))
						handle, spec := fakeWorkerClient.CreateContainerArgsForCall(0)
						Ω(handle).Should(Equal(sessionID))
						Ω(spec).Should(Equal(worker.ExecuteContainerSpec{
							Platform:   "some-platform",
							Tags:       []string{"some", "tags"},
							Image:      "some-image",
							Privileged: false,
						}))
					})

					It("ensures /tmp/build/src exists by streaming in an empty payload", func() {
						Ω(fakeContainer.StreamInCallCount()).Should(Equal(1))

						destination, stream := fakeContainer.StreamInArgsForCall(0)
						Ω(destination).Should(Equal("/tmp/build/src"))

						tarReader := tar.NewReader(stream)

						_, err := tarReader.Next()
						Ω(err).Should(Equal(io.EOF))
					})

					It("streams the input source in relative to /tmp/build/src", func() {
						Ω(inSource.StreamToCallCount()).Should(Equal(1))
						Ω(inSource.StreamToArgsForCall(0)).ShouldNot(BeNil())

						streamInCount := fakeContainer.StreamInCallCount()

						streamIn := new(bytes.Buffer)

						err := inSource.StreamToArgsForCall(0).StreamIn("some-path", streamIn)
						Ω(err).ShouldNot(HaveOccurred())

						destination, source := fakeContainer.StreamInArgsForCall(streamInCount)
						Ω(destination).Should(Equal("/tmp/build/src/some-path"))
						Ω(source).Should(Equal(streamIn))
					})

					It("runs a process with the config's path and args, in /tmp/build/src", func() {
						Ω(fakeContainer.RunCallCount()).Should(Equal(1))

						spec, _ := fakeContainer.RunArgsForCall(0)
						Ω(spec).Should(Equal(garden.ProcessSpec{
							Path:       "ls",
							Args:       []string{"some", "args"},
							Env:        []string{"SOME=params"},
							Dir:        "/tmp/build/src",
							Privileged: false,
							TTY:        &garden.TTYSpec{},
						}))
					})

					It("directs the process's stdout/stderr to the io config", func() {
						Ω(fakeContainer.RunCallCount()).Should(Equal(1))

						_, io := fakeContainer.RunArgsForCall(0)
						Ω(io.Stdout).Should(Equal(stdoutBuf))
						Ω(io.Stderr).Should(Equal(stderrBuf))
					})

					It("saves the process ID as a property", func() {
						Ω(fakeContainer.SetPropertyCallCount()).Should(Equal(1))

						name, value := fakeContainer.SetPropertyArgsForCall(0)
						Ω(name).Should(Equal("execute-process"))
						Ω(value).Should(Equal("42"))
					})

					It("invokes the delegate's Started callback", func() {
						Ω(executeDelegate.StartedCallCount()).Should(Equal(1))
					})

					Context("when privileged", func() {
						BeforeEach(func() {
							privileged = true
						})

						It("creates the container privileged", func() {
							Ω(fakeWorkerClient.CreateContainerCallCount()).Should(Equal(1))
							handle, spec := fakeWorkerClient.CreateContainerArgsForCall(0)
							Ω(handle).Should(Equal(sessionID))
							Ω(spec).Should(Equal(worker.ExecuteContainerSpec{
								Platform:   "some-platform",
								Tags:       []string{"some", "tags"},
								Image:      "some-image",
								Privileged: true,
							}))
						})

						It("runs the process privileged", func() {
							Ω(fakeContainer.RunCallCount()).Should(Equal(1))

							spec, _ := fakeContainer.RunArgsForCall(0)
							Ω(spec).Should(Equal(garden.ProcessSpec{
								Path:       "ls",
								Args:       []string{"some", "args"},
								Env:        []string{"SOME=params"},
								Dir:        "/tmp/build/src",
								Privileged: true,
								TTY:        &garden.TTYSpec{},
							}))
						})
					})

					Context("when the configuration specifies paths for inputs", func() {
						BeforeEach(func() {
							configSource.FetchConfigReturns(atc.BuildConfig{
								Image:  "some-image",
								Params: map[string]string{"SOME": "params"},
								Run: atc.BuildRunConfig{
									Path: "ls",
									Args: []string{"some", "args"},
								},
								Inputs: []atc.BuildInputConfig{
									{Name: "some-input", Path: "some-input-configured-path"},
									{Name: "some-other-input", Path: "some-other-input-configured-path"},
								},
							}, nil)
						})

						Context("when all inputs are present in the in source", func() {
							BeforeEach(func() {
								inSource.StreamToStub = func(dest ArtifactDestination) error {
									defer GinkgoRecover()

									streamIn := new(bytes.Buffer)

									By("remapping base destinations")
									err := dest.StreamIn("some-input", streamIn)
									Ω(err).ShouldNot(HaveOccurred())

									destination, source := fakeContainer.StreamInArgsForCall(1)
									Ω(destination).Should(Equal("/tmp/build/src/some-input-configured-path"))
									Ω(source).Should(Equal(streamIn))

									By("remapping subdirectory destinations")
									err = dest.StreamIn("some-input/some-thing", streamIn)
									Ω(err).ShouldNot(HaveOccurred())

									destination, source = fakeContainer.StreamInArgsForCall(2)
									Ω(destination).Should(Equal("/tmp/build/src/some-input-configured-path/some-thing"))
									Ω(source).Should(Equal(streamIn))

									By("remapping other base destinations")
									err = dest.StreamIn("some-other-input", streamIn)
									Ω(err).ShouldNot(HaveOccurred())

									destination, source = fakeContainer.StreamInArgsForCall(3)
									Ω(destination).Should(Equal("/tmp/build/src/some-other-input-configured-path"))
									Ω(source).Should(Equal(streamIn))

									By("not accidentally matching partial names")
									err = dest.StreamIn("some-input-morewords", streamIn)
									Ω(err).ShouldNot(HaveOccurred())

									destination, source = fakeContainer.StreamInArgsForCall(4)
									Ω(destination).Should(Equal("/tmp/build/src/some-input-morewords"))
									Ω(source).Should(Equal(streamIn))

									By("not remapping unconfigured destinations")
									err = dest.StreamIn("some-other-unconfigured-input", streamIn)
									Ω(err).ShouldNot(HaveOccurred())

									destination, source = fakeContainer.StreamInArgsForCall(5)
									Ω(destination).Should(Equal("/tmp/build/src/some-other-unconfigured-input"))
									Ω(source).Should(Equal(streamIn))

									return nil
								}
							})

							It("re-maps the stream destinations to the configured destinations", func() {
								Ω(inSource.StreamToCallCount()).Should(Equal(1))

								Eventually(process.Wait()).Should(Receive(BeNil()))
							})
						})

						Context("when any of the inputs are missing", func() {
							BeforeEach(func() {
								inSource.StreamToStub = func(dest ArtifactDestination) error {
									defer GinkgoRecover()

									streamIn := new(bytes.Buffer)

									err := dest.StreamIn("some-unconfigured-input", streamIn)
									Ω(err).ShouldNot(HaveOccurred())

									destination, source := fakeContainer.StreamInArgsForCall(1)
									Ω(destination).Should(Equal("/tmp/build/src/some-unconfigured-input"))
									Ω(source).Should(Equal(streamIn))

									return nil
								}
							})

							It("exits with failure", func() {
								Ω(inSource.StreamToCallCount()).Should(Equal(1))

								var err error
								Eventually(process.Wait()).Should(Receive(&err))
								Ω(err).Should(BeAssignableToTypeOf(MissingInputsError{}))
								Ω(err.(MissingInputsError).Inputs).Should(ConsistOf("some-input", "some-other-input"))
							})

							It("invokes the delegate's Failed callback", func() {
								Eventually(process.Wait()).Should(Receive(HaveOccurred()))

								Ω(executeDelegate.FailedCallCount()).Should(Equal(1))

								err := executeDelegate.FailedArgsForCall(0)
								Ω(err).Should(BeAssignableToTypeOf(MissingInputsError{}))
								Ω(err.(MissingInputsError).Inputs).Should(ConsistOf("some-input", "some-other-input"))
							})
						})
					})

					Context("when the configuration specifies names of inputs without paths", func() {
						BeforeEach(func() {
							configSource.FetchConfigReturns(atc.BuildConfig{
								Image:  "some-image",
								Params: map[string]string{"SOME": "params"},
								Run: atc.BuildRunConfig{
									Path: "ls",
									Args: []string{"some", "args"},
								},
								Inputs: []atc.BuildInputConfig{
									{Name: "some-input"},
									{Name: "some-other-input"},
								},
							}, nil)
						})

						Context("when all inputs are present in the in source", func() {
							BeforeEach(func() {
								inSource.StreamToStub = func(dest ArtifactDestination) error {
									defer GinkgoRecover()

									streamIn := new(bytes.Buffer)

									err := dest.StreamIn("some-input", streamIn)
									Ω(err).ShouldNot(HaveOccurred())

									destination, source := fakeContainer.StreamInArgsForCall(1)
									Ω(destination).Should(Equal("/tmp/build/src/some-input"))
									Ω(source).Should(Equal(streamIn))

									By("not remapping subdirectory destinations")
									err = dest.StreamIn("some-input/some-thing", streamIn)
									Ω(err).ShouldNot(HaveOccurred())

									destination, source = fakeContainer.StreamInArgsForCall(2)
									Ω(destination).Should(Equal("/tmp/build/src/some-input/some-thing"))
									Ω(source).Should(Equal(streamIn))

									By("not remapping other base destinations")
									err = dest.StreamIn("some-other-input", streamIn)
									Ω(err).ShouldNot(HaveOccurred())

									destination, source = fakeContainer.StreamInArgsForCall(3)
									Ω(destination).Should(Equal("/tmp/build/src/some-other-input"))
									Ω(source).Should(Equal(streamIn))

									By("not remapping unconfigured destinations")
									err = dest.StreamIn("some-other-unconfigured-input", streamIn)
									Ω(err).ShouldNot(HaveOccurred())

									destination, source = fakeContainer.StreamInArgsForCall(4)
									Ω(destination).Should(Equal("/tmp/build/src/some-other-unconfigured-input"))
									Ω(source).Should(Equal(streamIn))

									return nil
								}
							})

							It("does not re-map the stream destinations", func() {
								Ω(inSource.StreamToCallCount()).Should(Equal(1))

								Eventually(process.Wait()).Should(Receive(BeNil()))
							})
						})

						Context("when any of the inputs are missing", func() {
							BeforeEach(func() {
								inSource.StreamToStub = func(dest ArtifactDestination) error {
									defer GinkgoRecover()

									streamIn := new(bytes.Buffer)

									err := dest.StreamIn("some-unconfigured-input", streamIn)
									Ω(err).ShouldNot(HaveOccurred())

									destination, source := fakeContainer.StreamInArgsForCall(1)
									Ω(destination).Should(Equal("/tmp/build/src/some-unconfigured-input"))
									Ω(source).Should(Equal(streamIn))

									return nil
								}
							})

							It("exits with failure", func() {
								Ω(inSource.StreamToCallCount()).Should(Equal(1))

								var err error
								Eventually(process.Wait()).Should(Receive(&err))
								Ω(err).Should(BeAssignableToTypeOf(MissingInputsError{}))
								Ω(err.(MissingInputsError).Inputs).Should(ConsistOf("some-input", "some-other-input"))
							})

							It("invokes the delegate's Failed callback", func() {
								Eventually(process.Wait()).Should(Receive(HaveOccurred()))

								Ω(executeDelegate.FailedCallCount()).Should(Equal(1))

								err := executeDelegate.FailedArgsForCall(0)
								Ω(err).Should(BeAssignableToTypeOf(MissingInputsError{}))
								Ω(err.(MissingInputsError).Inputs).Should(ConsistOf("some-input", "some-other-input"))
							})
						})
					})

					Context("when the process exits 0", func() {
						BeforeEach(func() {
							fakeProcess.WaitReturns(0, nil)
						})

						It("saves the exit status property", func() {
							Eventually(process.Wait()).Should(Receive(BeNil()))

							Ω(fakeContainer.SetPropertyCallCount()).Should(Equal(2))

							name, value := fakeContainer.SetPropertyArgsForCall(1)
							Ω(name).Should(Equal("exit-status"))
							Ω(value).Should(Equal("0"))
						})

						It("is successful", func() {
							Eventually(process.Wait()).Should(Receive(BeNil()))

							var success Success
							Ω(source.Result(&success)).Should(BeTrue())
							Ω(bool(success)).Should(BeTrue())
						})

						It("reports its exit status", func() {
							Eventually(process.Wait()).Should(Receive(BeNil()))

							var status ExitStatus
							Ω(source.Result(&status)).Should(BeTrue())
							Ω(status).Should(Equal(ExitStatus(0)))
						})

						Describe("before saving the exit status property", func() {
							BeforeEach(func() {
								executeDelegate.FinishedStub = func(ExitStatus) {
									callCount := fakeContainer.SetPropertyCallCount()

									for i := 0; i < callCount; i++ {
										name, _ := fakeContainer.SetPropertyArgsForCall(i)
										Ω(name).ShouldNot(Equal("exit-status"))
									}
								}
							})

							It("invokes the delegate's Finished callback", func() {
								Eventually(process.Wait()).Should(Receive(BeNil()))

								Ω(executeDelegate.FinishedCallCount()).Should(Equal(1))
								Ω(executeDelegate.FinishedArgsForCall(0)).Should(Equal(ExitStatus(0)))
							})
						})

						Context("when saving the exit status fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeContainer.SetPropertyStub = func(name string, value string) error {
									if name == "exit-status" {
										return disaster
									}

									return nil
								}
							})

							It("exits with the error", func() {
								Eventually(process.Wait()).Should(Receive(Equal(disaster)))
							})

							It("invokes the delegate's Failed callback", func() {
								Eventually(process.Wait()).Should(Receive(Equal(disaster)))
								Ω(executeDelegate.FailedCallCount()).Should(Equal(1))
								Ω(executeDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
							})
						})
					})

					Context("when the process exits nonzero", func() {
						BeforeEach(func() {
							fakeProcess.WaitReturns(1, nil)
						})

						It("saves the exit status property", func() {
							Eventually(process.Wait()).Should(Receive(BeNil()))

							Ω(fakeContainer.SetPropertyCallCount()).Should(Equal(2))

							name, value := fakeContainer.SetPropertyArgsForCall(1)
							Ω(name).Should(Equal("exit-status"))
							Ω(value).Should(Equal("1"))
						})

						It("is not successful", func() {
							Eventually(process.Wait()).Should(Receive(BeNil()))

							var success Success
							Ω(source.Result(&success)).Should(BeTrue())
							Ω(bool(success)).Should(BeFalse())
						})

						It("reports its exit status", func() {
							Eventually(process.Wait()).Should(Receive(BeNil()))

							var status ExitStatus
							Ω(source.Result(&status)).Should(BeTrue())
							Ω(status).Should(Equal(ExitStatus(1)))
						})

						Describe("before saving the exit status property", func() {
							BeforeEach(func() {
								executeDelegate.FinishedStub = func(ExitStatus) {
									callCount := fakeContainer.SetPropertyCallCount()

									for i := 0; i < callCount; i++ {
										name, _ := fakeContainer.SetPropertyArgsForCall(i)
										Ω(name).ShouldNot(Equal("exit-status"))
									}
								}
							})

							It("invokes the delegate's Finished callback", func() {
								Eventually(process.Wait()).Should(Receive(BeNil()))

								Ω(executeDelegate.FinishedCallCount()).Should(Equal(1))
								Ω(executeDelegate.FinishedArgsForCall(0)).Should(Equal(ExitStatus(1)))
							})
						})

						Context("when saving the exit status fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeContainer.SetPropertyStub = func(name string, value string) error {
									if name == "exit-status" {
										return disaster
									}

									return nil
								}
							})

							It("exits with the error", func() {
								Eventually(process.Wait()).Should(Receive(Equal(disaster)))
							})

							It("invokes the delegate's Failed callback", func() {
								Eventually(process.Wait()).Should(Receive(Equal(disaster)))
								Ω(executeDelegate.FailedCallCount()).Should(Equal(1))
								Ω(executeDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
							})
						})
					})

					Context("when waiting on the process fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeProcess.WaitReturns(0, disaster)
						})

						It("exits with the failure", func() {
							Eventually(process.Wait()).Should(Receive(Equal(disaster)))
						})

						It("invokes the delegate's Failed callback", func() {
							Eventually(process.Wait()).Should(Receive(Equal(disaster)))
							Ω(executeDelegate.FailedCallCount()).Should(Equal(1))
							Ω(executeDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
						})
					})

					Context("when setting the process property fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeContainer.SetPropertyReturns(disaster)
						})

						It("exits with the error", func() {
							Eventually(process.Wait()).Should(Receive(Equal(disaster)))
						})

						It("invokes the delegate's Failed callback", func() {
							Eventually(process.Wait()).Should(Receive(Equal(disaster)))
							Ω(executeDelegate.FailedCallCount()).Should(Equal(1))
							Ω(executeDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
						})
					})

					Describe("streaming to a destination", func() {
						var fakeDestination *fakes.FakeArtifactDestination

						BeforeEach(func() {
							fakeDestination = new(fakes.FakeArtifactDestination)
						})

						Context("when the resource can stream out", func() {
							var streamedOut io.ReadCloser

							BeforeEach(func() {
								streamedOut = gbytes.NewBuffer()
								fakeContainer.StreamOutReturns(streamedOut, nil)
							})

							It("streams the resource to the destination", func() {
								err := source.StreamTo(fakeDestination)
								Ω(err).ShouldNot(HaveOccurred())

								Ω(fakeContainer.StreamOutCallCount()).Should(Equal(1))
								Ω(fakeContainer.StreamOutArgsForCall(0)).Should(Equal("/tmp/build/src/"))

								Ω(fakeDestination.StreamInCallCount()).Should(Equal(1))
								dest, src := fakeDestination.StreamInArgsForCall(0)
								Ω(dest).Should(Equal("."))
								Ω(src).Should(Equal(streamedOut))
							})

							Context("when streaming out of the versioned source fails", func() {
								disaster := errors.New("nope")

								BeforeEach(func() {
									fakeContainer.StreamOutReturns(nil, disaster)
								})

								It("returns the error", func() {
									Ω(source.StreamTo(fakeDestination)).Should(Equal(disaster))
								})
							})

							Context("when streaming in to the destination fails", func() {
								disaster := errors.New("nope")

								BeforeEach(func() {
									fakeDestination.StreamInReturns(disaster)
								})

								It("returns the error", func() {
									Ω(source.StreamTo(fakeDestination)).Should(Equal(disaster))
								})
							})
						})

						Context("when the container cannot stream out", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeContainer.StreamOutReturns(nil, disaster)
							})

							It("returns the error", func() {
								Ω(source.StreamTo(fakeDestination)).Should(Equal(disaster))
							})
						})
					})

					Describe("streaming a file out", func() {
						Context("when the container can stream out", func() {
							var (
								fileContent = "file-content"

								tarBuffer *gbytes.Buffer
							)

							BeforeEach(func() {
								tarBuffer = gbytes.NewBuffer()
								fakeContainer.StreamOutReturns(tarBuffer, nil)
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
									reader, err := source.StreamFile("some-path")
									Ω(err).ShouldNot(HaveOccurred())

									Ω(ioutil.ReadAll(reader)).Should(Equal([]byte(fileContent)))

									Ω(fakeContainer.StreamOutArgsForCall(0)).Should(Equal("/tmp/build/src/some-path"))
								})

								Describe("closing the stream", func() {
									It("closes the stream from the versioned source", func() {
										reader, err := source.StreamFile("some-path")
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
									_, err := source.StreamFile("some-path")
									Ω(err).Should(Equal(ErrFileNotFound))
								})
							})
						})

						Context("when the container cannot stream out", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeContainer.StreamOutReturns(nil, disaster)
							})

							It("returns the error", func() {
								_, err := source.StreamFile("some-path")
								Ω(err).Should(Equal(disaster))
							})
						})
					})

					Describe("signalling", func() {
						var stopped chan struct{}

						BeforeEach(func() {
							stopped = make(chan struct{})

							fakeProcess.WaitStub = func() (int, error) {
								<-stopped
								return 128 + 15, nil
							}

							fakeContainer.StopStub = func(bool) error {
								close(stopped)
								return nil
							}
						})

						It("stops the container", func() {
							process.Signal(os.Interrupt)
							Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))

							Ω(fakeContainer.StopCallCount()).Should(Equal(1))
						})
					})

					Describe("releasing", func() {
						It("releases the container", func() {
							Ω(fakeContainer.ReleaseCallCount()).Should(BeZero())

							err := source.Release()
							Ω(err).ShouldNot(HaveOccurred())

							Ω(fakeContainer.ReleaseCallCount()).Should(Equal(1))
						})
					})

					Context("when streaming out from the previous source fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							inSource.StreamToReturns(disaster)
						})

						It("exits with the error", func() {
							Eventually(process.Wait()).Should(Receive(Equal(disaster)))
						})

						It("invokes the delegate's Failed callback", func() {
							Eventually(process.Wait()).Should(Receive(Equal(disaster)))
							Ω(executeDelegate.FailedCallCount()).Should(Equal(1))
							Ω(executeDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
						})
					})

					Context("when streaming the bits in to the container fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							inSource.StreamToReturns(disaster)
						})

						It("exits with the error", func() {
							Eventually(process.Wait()).Should(Receive(Equal(disaster)))
						})

						It("does not execute anything", func() {
							Eventually(process.Wait()).Should(Receive())
							Ω(fakeContainer.RunCallCount()).Should(Equal(0))
						})

						It("invokes the delegate's Failed callback", func() {
							Eventually(process.Wait()).Should(Receive(Equal(disaster)))
							Ω(executeDelegate.FailedCallCount()).Should(Equal(1))
							Ω(executeDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
						})
					})

					Context("when running the build's script fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeContainer.RunReturns(nil, disaster)
						})

						It("exits with the error", func() {
							Eventually(process.Wait()).Should(Receive(Equal(disaster)))
						})

						It("invokes the delegate's Failed callback", func() {
							Eventually(process.Wait()).Should(Receive(Equal(disaster)))
							Ω(executeDelegate.FailedCallCount()).Should(Equal(1))
							Ω(executeDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
						})
					})
				})

				Context("when creating the container fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeWorkerClient.CreateContainerReturns(nil, disaster)
					})

					It("exits with the error", func() {
						Eventually(process.Wait()).Should(Receive(Equal(disaster)))
					})

					It("invokes the delegate's Failed callback", func() {
						Eventually(process.Wait()).Should(Receive(Equal(disaster)))
						Ω(executeDelegate.FailedCallCount()).Should(Equal(1))
						Ω(executeDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
					})

					Describe("releasing", func() {
						JustBeforeEach(func() {
							Eventually(process.Wait()).Should(Receive(Equal(disaster)))
						})

						It("succeeds", func() {
							err := source.Release()
							Ω(err).ShouldNot(HaveOccurred())
						})
					})
				})
			})

			Context("when getting the config fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					configSource.FetchConfigReturns(atc.BuildConfig{}, disaster)
				})

				It("exits with the failure", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
				})

				It("invokes the delegate's Failed callback", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
					Ω(executeDelegate.FailedCallCount()).Should(Equal(1))
					Ω(executeDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
				})
			})
		})

		Context("when the container already exists", func() {
			var fakeContainer *wfakes.FakeContainer

			BeforeEach(func() {
				fakeContainer = new(wfakes.FakeContainer)
				fakeWorkerClient.LookupReturns(fakeContainer, nil)
			})

			Context("when an exit status is already saved off", func() {
				BeforeEach(func() {
					fakeContainer.GetPropertyStub = func(name string) (string, error) {
						switch name {
						case "exit-status":
							return "123", nil
						default:
							return "", errors.New("unstubbed property: " + name)
						}
					}
				})

				It("exits with success", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))
				})

				It("does not attach to any process", func() {
					Ω(fakeContainer.AttachCallCount()).Should(BeZero())
				})

				It("is not successful", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))

					var success Success
					Ω(source.Result(&success)).Should(BeTrue())
					Ω(bool(success)).Should(BeFalse())
				})

				It("reports its exit status", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))

					var status ExitStatus
					Ω(source.Result(&status)).Should(BeTrue())
					Ω(status).Should(Equal(ExitStatus(123)))
				})

				It("does not invoke the delegate's Started callback", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))
					Ω(executeDelegate.StartedCallCount()).Should(BeZero())
				})

				It("does not invoke the delegate's Finished callback", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))
					Ω(executeDelegate.FinishedCallCount()).Should(BeZero())
				})
			})

			Context("when the process id can be found", func() {
				BeforeEach(func() {
					fakeContainer.GetPropertyStub = func(name string) (string, error) {
						switch name {
						case "execute-process":
							return "42", nil
						default:
							return "", errors.New("unstubbed property: " + name)
						}
					}
				})

				Context("when attaching to the process succeeds", func() {
					var fakeProcess *gfakes.FakeProcess

					BeforeEach(func() {
						fakeProcess = new(gfakes.FakeProcess)
						fakeContainer.AttachReturns(fakeProcess, nil)
					})

					It("attaches to the correct process", func() {
						Ω(fakeContainer.AttachCallCount()).Should(Equal(1))

						pid, _ := fakeContainer.AttachArgsForCall(0)
						Ω(pid).Should(Equal(uint32(42)))
					})

					It("directs the process's stdout/stderr to the io config", func() {
						Ω(fakeContainer.AttachCallCount()).Should(Equal(1))

						_, pio := fakeContainer.AttachArgsForCall(0)
						Ω(pio.Stdout).Should(Equal(stdoutBuf))
						Ω(pio.Stderr).Should(Equal(stderrBuf))
					})

					It("does not invoke the delegate's Started callback", func() {
						Ω(executeDelegate.StartedCallCount()).Should(BeZero())
					})
				})

				Context("when attaching to the process fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeContainer.AttachReturns(nil, disaster)
					})

					It("exits with the error", func() {
						Eventually(process.Wait()).Should(Receive(Equal(disaster)))
					})

					It("invokes the delegate's Failed callback", func() {
						Eventually(process.Wait()).Should(Receive(Equal(disaster)))
						Ω(executeDelegate.FailedCallCount()).Should(Equal(1))
						Ω(executeDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
					})
				})
			})

			Context("when the process id cannot be found", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeContainer.GetPropertyReturns("", disaster)
				})

				It("exits with the failure", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
				})

				It("invokes the delegate's Failed callback", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
					Eventually(executeDelegate.FailedCallCount()).Should(Equal(1))
					Ω(executeDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
				})
			})
		})
	})
})
