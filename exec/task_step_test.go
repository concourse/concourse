package exec_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"

	"github.com/cloudfoundry-incubator/garden"
	gfakes "github.com/cloudfoundry-incubator/garden/fakes"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/fakes"
	rfakes "github.com/concourse/atc/resource/fakes"
	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/fakes"
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

		sourceName SourceName = "some-source-name"

		identifier = worker.Identifier{
			ContainerIdentifier: db.ContainerIdentifier{
				Name: "some-session-id",
			},
		}
	)

	BeforeEach(func() {
		fakeTrackerFactory = new(fakes.FakeTrackerFactory)

		fakeTracker = new(rfakes.FakeTracker)
		fakeTrackerFactory.TrackerForReturns(fakeTracker)

		fakeWorkerClient = new(wfakes.FakeClient)

		factory = NewGardenFactory(fakeWorkerClient, fakeTrackerFactory, func() string { return "a-random-guid" })

		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
	})

	Describe("Task", func() {
		var (
			taskDelegate *fakes.FakeTaskDelegate
			privileged   Privileged
			tags         []string
			configSource *fakes.FakeTaskConfigSource

			inStep *fakes.FakeStep
			repo   *SourceRepository

			step    Step
			process ifrit.Process
		)

		BeforeEach(func() {
			taskDelegate = new(fakes.FakeTaskDelegate)
			taskDelegate.StdoutReturns(stdoutBuf)
			taskDelegate.StderrReturns(stderrBuf)

			privileged = false
			tags = []string{"step", "tags"}
			configSource = new(fakes.FakeTaskConfigSource)

			inStep = new(fakes.FakeStep)
			repo = NewSourceRepository()
		})

		JustBeforeEach(func() {
			step = factory.Task(
				lagertest.NewTestLogger("test"),
				sourceName,
				identifier,
				taskDelegate,
				privileged,
				tags,
				configSource,
			).Using(inStep, repo)

			process = ifrit.Invoke(step)
		})

		Context("when the container does not yet exist", func() {
			BeforeEach(func() {
				fakeWorkerClient.FindContainerForIdentifierReturns(nil, false, errors.New("nope"))
			})

			Context("when the getting the config works", func() {
				var fetchedConfig atc.TaskConfig

				BeforeEach(func() {
					fetchedConfig = atc.TaskConfig{
						Platform: "some-platform",
						Tags:     []string{"config", "tags"},
						Image:    "some-image",
						Params:   map[string]string{"SOME": "params"},
						Run: atc.TaskRunConfig{
							Path: "ls",
							Args: []string{"some", "args"},
						},
					}

					configSource.FetchConfigReturns(fetchedConfig, nil)
				})

				Context("when a worker can not be located", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeWorkerClient.SatisfyingReturns(nil, disaster)
					})

					It("exits with the error", func() {
						Expect(<-process.Wait()).To(Equal(disaster))
					})
				})

				Context("when a worker can be located", func() {
					var fakeWorker *wfakes.FakeWorker
					var fakeBaggageclaimClient *bfakes.FakeClient

					BeforeEach(func() {
						fakeWorker = new(wfakes.FakeWorker)

						fakeBaggageclaimClient = new(bfakes.FakeClient)
						fakeWorker.VolumeManagerReturns(fakeBaggageclaimClient, true)

						expectedSpec := worker.WorkerSpec{
							Platform: "some-platform",
							Tags:     []string{"step", "tags"},
						}

						fakeWorkerClient.SatisfyingStub = func(spec worker.WorkerSpec) (worker.Worker, error) {
							if reflect.DeepEqual(spec, expectedSpec) {
								return fakeWorker, nil
							} else {
								return nil, fmt.Errorf("unexpected spec: %#v\n", spec)
							}
						}
					})

					Context("when creating the task's container works", func() {
						var (
							fakeContainer *wfakes.FakeContainer
							fakeProcess   *gfakes.FakeProcess
						)

						BeforeEach(func() {
							fakeContainer = new(wfakes.FakeContainer)
							fakeContainer.HandleReturns("some-handle")
							fakeWorker.CreateContainerReturns(fakeContainer, nil)

							fakeProcess = new(gfakes.FakeProcess)
							fakeProcess.IDReturns(42)
							fakeContainer.RunReturns(fakeProcess, nil)

							fakeContainer.StreamInReturns(nil)
						})

						Describe("before having created the container", func() {
							BeforeEach(func() {
								taskDelegate.InitializingStub = func(atc.TaskConfig) {
									defer GinkgoRecover()
									Expect(fakeWorker.CreateContainerCallCount()).To(BeZero())
								}
							})

							It("invokes the delegate's Initializing callback", func() {
								Expect(taskDelegate.InitializingCallCount()).To(Equal(1))
								Expect(taskDelegate.InitializingArgsForCall(0)).To(Equal(fetchedConfig))
							})
						})

						It("looked up the container via the session ID across the entire pool", func() {
							_, findID := fakeWorkerClient.FindContainerForIdentifierArgsForCall(0)
							Expect(findID).To(Equal(identifier))
						})

						It("gets the config from the input artifact soruce", func() {
							Expect(configSource.FetchConfigCallCount()).To(Equal(1))
							Expect(configSource.FetchConfigArgsForCall(0)).To(Equal(repo))
						})

						It("creates a container with the config's image and the session ID as the handle", func() {
							Expect(fakeWorker.CreateContainerCallCount()).To(Equal(1))
							_, createdIdentifier, spec := fakeWorker.CreateContainerArgsForCall(0)
							Expect(createdIdentifier).To(Equal(identifier))

							taskSpec := spec.(worker.TaskContainerSpec)
							Expect(taskSpec.Platform).To(Equal("some-platform"))
							Expect(taskSpec.Tags).To(ConsistOf("config", "step", "tags"))
							Expect(taskSpec.Image).To(Equal("some-image"))
							Expect(taskSpec.Privileged).To(BeFalse())
						})

						It("ensures artifacts root exists by streaming in an empty payload", func() {
							Expect(fakeContainer.StreamInCallCount()).To(Equal(1))

							spec := fakeContainer.StreamInArgsForCall(0)
							Expect(spec.Path).To(Equal("/tmp/build/a-random-guid"))
							Expect(spec.User).To(Equal("")) // use default

							tarReader := tar.NewReader(spec.TarStream)

							_, err := tarReader.Next()
							Expect(err).To(Equal(io.EOF))
						})

						It("runs a process with the config's path and args, in the specified build directory", func() {
							Expect(fakeContainer.RunCallCount()).To(Equal(1))

							spec, _ := fakeContainer.RunArgsForCall(0)
							Expect(spec.Path).To(Equal("ls"))
							Expect(spec.Args).To(Equal([]string{"some", "args"}))
							Expect(spec.Env).To(Equal([]string{"SOME=params"}))
							Expect(spec.Dir).To(Equal("/tmp/build/a-random-guid"))
							Expect(spec.User).To(Equal("root"))
							Expect(spec.TTY).To(Equal(&garden.TTYSpec{}))
						})

						It("directs the process's stdout/stderr to the io config", func() {
							Expect(fakeContainer.RunCallCount()).To(Equal(1))

							_, io := fakeContainer.RunArgsForCall(0)
							Expect(io.Stdout).To(Equal(stdoutBuf))
							Expect(io.Stderr).To(Equal(stderrBuf))
						})

						It("saves the process ID as a property", func() {
							Expect(fakeContainer.SetPropertyCallCount()).NotTo(Equal(0))

							name, value := fakeContainer.SetPropertyArgsForCall(0)
							Expect(name).To(Equal("concourse:task-process"))
							Expect(value).To(Equal("42"))
						})

						It("invokes the delegate's Started callback", func() {
							Expect(taskDelegate.StartedCallCount()).To(Equal(1))
						})

						Context("when privileged", func() {
							BeforeEach(func() {
								privileged = true
							})

							It("creates the container privileged", func() {
								Expect(fakeWorker.CreateContainerCallCount()).To(Equal(1))
								_, createdIdentifier, spec := fakeWorker.CreateContainerArgsForCall(0)
								Expect(createdIdentifier).To(Equal(identifier))

								taskSpec := spec.(worker.TaskContainerSpec)
								Expect(taskSpec.Platform).To(Equal("some-platform"))
								Expect(taskSpec.Tags).To(ConsistOf("config", "step", "tags"))
								Expect(taskSpec.Image).To(Equal("some-image"))
								Expect(taskSpec.Privileged).To(BeTrue())
							})

							It("runs the process as root", func() {
								Expect(fakeContainer.RunCallCount()).To(Equal(1))

								spec, _ := fakeContainer.RunArgsForCall(0)
								Expect(spec).To(Equal(garden.ProcessSpec{
									Path: "ls",
									Args: []string{"some", "args"},
									Env:  []string{"SOME=params"},
									Dir:  "/tmp/build/a-random-guid",
									User: "root",
									TTY:  &garden.TTYSpec{},
								}))

							})
						})

						Context("when the configuration specifies paths for inputs", func() {
							var inputSource *fakes.FakeArtifactSource
							var otherInputSource *fakes.FakeArtifactSource

							BeforeEach(func() {
								inputSource = new(fakes.FakeArtifactSource)
								otherInputSource = new(fakes.FakeArtifactSource)

								configSource.FetchConfigReturns(atc.TaskConfig{
									Platform: "some-platform",
									Image:    "some-image",
									Params:   map[string]string{"SOME": "params"},
									Run: atc.TaskRunConfig{
										Path: "ls",
										Args: []string{"some", "args"},
									},
									Inputs: []atc.TaskInputConfig{
										{Name: "some-input", Path: "some-input-configured-path"},
										{Name: "some-other-input"},
									},
								}, nil)
							})

							Context("when all inputs are present in the in source repository", func() {
								BeforeEach(func() {
									repo.RegisterSource("some-input", inputSource)
									repo.RegisterSource("some-other-input", otherInputSource)
								})

								It("streams each of them to their configured destinations", func() {
									streamIn := new(bytes.Buffer)

									Expect(inputSource.StreamToCallCount()).To(Equal(1))

									destination := inputSource.StreamToArgsForCall(0)

									initial := fakeContainer.StreamInCallCount()

									err := destination.StreamIn("foo", streamIn)
									Expect(err).NotTo(HaveOccurred())

									Expect(fakeContainer.StreamInCallCount()).To(Equal(initial + 1))

									spec := fakeContainer.StreamInArgsForCall(initial)
									Expect(spec.Path).To(Equal("/tmp/build/a-random-guid/some-input-configured-path/foo"))
									Expect(spec.User).To(Equal("")) // use default
									Expect(spec.TarStream).To(Equal(streamIn))

									Expect(otherInputSource.StreamToCallCount()).To(Equal(1))

									destination = otherInputSource.StreamToArgsForCall(0)

									initial = fakeContainer.StreamInCallCount()

									err = destination.StreamIn("foo", streamIn)
									Expect(err).NotTo(HaveOccurred())

									Expect(fakeContainer.StreamInCallCount()).To(Equal(initial + 1))
									spec = fakeContainer.StreamInArgsForCall(initial)
									Expect(spec.Path).To(Equal("/tmp/build/a-random-guid/some-other-input/foo"))
									Expect(spec.User).To(Equal("")) // use default
									Expect(spec.TarStream).To(Equal(streamIn))

									Eventually(process.Wait()).Should(Receive(BeNil()))
								})

								Context("when the inputs have volumes on the chosen worker", func() {
									var inputVolume *bfakes.FakeVolume
									var otherInputVolume *bfakes.FakeVolume

									BeforeEach(func() {
										inputVolume = new(bfakes.FakeVolume)
										inputVolume.HandleReturns("input-volume")

										otherInputVolume = new(bfakes.FakeVolume)
										otherInputVolume.HandleReturns("other-input-volume")

										inputSource.VolumeOnReturns(inputVolume, true, nil)
										otherInputSource.VolumeOnReturns(otherInputVolume, true, nil)
									})

									It("bind-mounts copy-on-write volumes to their destinations in the container", func() {
										_, _, spec := fakeWorker.CreateContainerArgsForCall(0)
										taskSpec := spec.(worker.TaskContainerSpec)
										Expect(taskSpec.Inputs).To(Equal([]worker.VolumeMount{
											{
												Volume:    inputVolume,
												MountPath: "/tmp/build/a-random-guid/some-input-configured-path",
											},
											{
												Volume:    otherInputVolume,
												MountPath: "/tmp/build/a-random-guid/some-other-input",
											},
										}))
									})

									It("releases the volumes given to the worker", func() {
										Expect(inputVolume.ReleaseCallCount()).To(Equal(1))
										Expect(otherInputVolume.ReleaseCallCount()).To(Equal(1))
									})

									It("does not stream inputs that had volumes", func() {
										Expect(inputSource.StreamToCallCount()).To(Equal(0))
										Expect(otherInputSource.StreamToCallCount()).To(Equal(0))
									})
								})

								Context("when streaming the bits in to the container fails", func() {
									disaster := errors.New("nope")

									BeforeEach(func() {
										inputSource.StreamToReturns(disaster)
									})

									It("exits with the error", func() {
										Eventually(process.Wait()).Should(Receive(Equal(disaster)))
									})

									It("does not run anything", func() {
										Eventually(process.Wait()).Should(Receive())
										Expect(fakeContainer.RunCallCount()).To(Equal(0))
									})

									It("invokes the delegate's Failed callback", func() {
										Eventually(process.Wait()).Should(Receive(Equal(disaster)))
										Expect(taskDelegate.FailedCallCount()).To(Equal(1))
										Expect(taskDelegate.FailedArgsForCall(0)).To(Equal(disaster))
									})
								})
							})

							Context("when any of the inputs are missing", func() {
								BeforeEach(func() {
									repo.RegisterSource("some-input", inputSource)
									// repo.RegisterSource("some-other-input", otherInputSource)
								})

								It("exits with failure", func() {
									var err error
									Eventually(process.Wait()).Should(Receive(&err))
									Expect(err).To(BeAssignableToTypeOf(MissingInputsError{}))
									Expect(err.(MissingInputsError).Inputs).To(ConsistOf("some-other-input"))
								})

								It("invokes the delegate's Failed callback", func() {
									Eventually(process.Wait()).Should(Receive(HaveOccurred()))

									Expect(taskDelegate.FailedCallCount()).To(Equal(1))

									err := taskDelegate.FailedArgsForCall(0)
									Expect(err).To(BeAssignableToTypeOf(MissingInputsError{}))
									Expect(err.(MissingInputsError).Inputs).To(ConsistOf("some-other-input"))
								})
							})
						})

						Context("when the process exits 0", func() {
							BeforeEach(func() {
								fakeProcess.WaitReturns(0, nil)
							})

							It("saves the exit status property", func() {
								Eventually(process.Wait()).Should(Receive(BeNil()))

								Expect(fakeContainer.SetPropertyCallCount()).To(Equal(2))

								name, value := fakeContainer.SetPropertyArgsForCall(1)
								Expect(name).To(Equal("concourse:exit-status"))
								Expect(value).To(Equal("0"))
							})

							It("is successful", func() {
								Eventually(process.Wait()).Should(Receive(BeNil()))

								var success Success
								Expect(step.Result(&success)).To(BeTrue())
								Expect(bool(success)).To(BeTrue())
							})

							It("reports its exit status", func() {
								Eventually(process.Wait()).Should(Receive(BeNil()))

								var status ExitStatus
								Expect(step.Result(&status)).To(BeTrue())
								Expect(status).To(Equal(ExitStatus(0)))
							})

							Describe("the registered source", func() {
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
										var streamedOut io.ReadCloser

										BeforeEach(func() {
											streamedOut = gbytes.NewBuffer()
											fakeContainer.StreamOutReturns(streamedOut, nil)
										})

										It("streams the resource to the destination", func() {
											err := artifactSource.StreamTo(fakeDestination)
											Expect(err).NotTo(HaveOccurred())

											Expect(fakeContainer.StreamOutCallCount()).To(Equal(1))
											spec := fakeContainer.StreamOutArgsForCall(0)
											Expect(spec.Path).To(Equal("/tmp/build/a-random-guid/"))
											Expect(spec.User).To(Equal("")) // use default

											Expect(fakeDestination.StreamInCallCount()).To(Equal(1))
											dest, src := fakeDestination.StreamInArgsForCall(0)
											Expect(dest).To(Equal("."))
											Expect(src).To(Equal(streamedOut))
										})

										Context("when streaming out of the versioned source fails", func() {
											disaster := errors.New("nope")

											BeforeEach(func() {
												fakeContainer.StreamOutReturns(nil, disaster)
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

									Context("when the container cannot stream out", func() {
										disaster := errors.New("nope")

										BeforeEach(func() {
											fakeContainer.StreamOutReturns(nil, disaster)
										})

										It("returns the error", func() {
											Expect(artifactSource.StreamTo(fakeDestination)).To(Equal(disaster))
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
												Expect(err).NotTo(HaveOccurred())

												_, err = tarWriter.Write([]byte(fileContent))
												Expect(err).NotTo(HaveOccurred())
											})

											It("streams out the given path", func() {
												reader, err := artifactSource.StreamFile("some-path")
												Expect(err).NotTo(HaveOccurred())

												Expect(ioutil.ReadAll(reader)).To(Equal([]byte(fileContent)))

												spec := fakeContainer.StreamOutArgsForCall(0)
												Expect(spec.Path).To(Equal("/tmp/build/a-random-guid/some-path"))
												Expect(spec.User).To(Equal("")) // use default
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

									Context("when the container cannot stream out", func() {
										disaster := errors.New("nope")

										BeforeEach(func() {
											fakeContainer.StreamOutReturns(nil, disaster)
										})

										It("returns the error", func() {
											_, err := artifactSource.StreamFile("some-path")
											Expect(err).To(Equal(disaster))
										})
									})
								})
							})

							Describe("before saving the exit status property", func() {
								BeforeEach(func() {
									taskDelegate.FinishedStub = func(ExitStatus) {
										defer GinkgoRecover()

										callCount := fakeContainer.SetPropertyCallCount()

										for i := 0; i < callCount; i++ {
											name, _ := fakeContainer.SetPropertyArgsForCall(i)
											Expect(name).NotTo(Equal("concourse:exit-status"))
										}
									}
								})

								It("invokes the delegate's Finished callback", func() {
									Eventually(process.Wait()).Should(Receive(BeNil()))

									Expect(taskDelegate.FinishedCallCount()).To(Equal(1))
									Expect(taskDelegate.FinishedArgsForCall(0)).To(Equal(ExitStatus(0)))
								})
							})

							Context("when saving the exit status fails", func() {
								disaster := errors.New("nope")

								BeforeEach(func() {
									fakeContainer.SetPropertyStub = func(name string, value string) error {
										defer GinkgoRecover()

										if name == "concourse:exit-status" {
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
									Expect(taskDelegate.FailedCallCount()).To(Equal(1))
									Expect(taskDelegate.FailedArgsForCall(0)).To(Equal(disaster))
								})
							})
						})

						Context("when the process exits nonzero", func() {
							BeforeEach(func() {
								fakeProcess.WaitReturns(1, nil)
							})

							It("saves the exit status property", func() {
								Eventually(process.Wait()).Should(Receive(BeNil()))

								Expect(fakeContainer.SetPropertyCallCount()).To(Equal(2))

								name, value := fakeContainer.SetPropertyArgsForCall(1)
								Expect(name).To(Equal("concourse:exit-status"))
								Expect(value).To(Equal("1"))
							})

							It("is not successful", func() {
								Eventually(process.Wait()).Should(Receive(BeNil()))

								var success Success
								Expect(step.Result(&success)).To(BeTrue())
								Expect(bool(success)).To(BeFalse())
							})

							It("reports its exit status", func() {
								Eventually(process.Wait()).Should(Receive(BeNil()))

								var status ExitStatus
								Expect(step.Result(&status)).To(BeTrue())
								Expect(status).To(Equal(ExitStatus(1)))
							})

							Describe("before saving the exit status property", func() {
								BeforeEach(func() {
									taskDelegate.FinishedStub = func(ExitStatus) {
										defer GinkgoRecover()

										callCount := fakeContainer.SetPropertyCallCount()

										for i := 0; i < callCount; i++ {
											name, _ := fakeContainer.SetPropertyArgsForCall(i)
											Expect(name).NotTo(Equal("concourse:exit-status"))
										}
									}
								})

								It("invokes the delegate's Finished callback", func() {
									Eventually(process.Wait()).Should(Receive(BeNil()))

									Expect(taskDelegate.FinishedCallCount()).To(Equal(1))
									Expect(taskDelegate.FinishedArgsForCall(0)).To(Equal(ExitStatus(1)))
								})
							})

							Context("when saving the exit status fails", func() {
								disaster := errors.New("nope")

								BeforeEach(func() {
									fakeContainer.SetPropertyStub = func(name string, value string) error {
										defer GinkgoRecover()

										if name == "concourse:exit-status" {
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
									Expect(taskDelegate.FailedCallCount()).To(Equal(1))
									Expect(taskDelegate.FailedArgsForCall(0)).To(Equal(disaster))
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
								Expect(taskDelegate.FailedCallCount()).To(Equal(1))
								Expect(taskDelegate.FailedArgsForCall(0)).To(Equal(disaster))
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
								Expect(taskDelegate.FailedCallCount()).To(Equal(1))
								Expect(taskDelegate.FailedArgsForCall(0)).To(Equal(disaster))
							})
						})

						Describe("signalling", func() {
							var stopped chan struct{}

							BeforeEach(func() {
								stopped = make(chan struct{})

								fakeProcess.WaitStub = func() (int, error) {
									defer GinkgoRecover()

									<-stopped
									return 128 + 15, nil
								}

								fakeContainer.StopStub = func(bool) error {
									defer GinkgoRecover()

									close(stopped)
									return nil
								}
							})

							It("stops the container", func() {
								process.Signal(os.Interrupt)
								Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))

								Expect(fakeContainer.StopCallCount()).To(Equal(1))
							})
						})

						Describe("releasing", func() {
							It("releases the container", func() {
								Expect(fakeContainer.ReleaseCallCount()).To(BeZero())

								step.Release()
								Expect(fakeContainer.ReleaseCallCount()).To(Equal(1))
							})
						})

						Context("when running the task's script fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeContainer.RunReturns(nil, disaster)
							})

							It("exits with the error", func() {
								Eventually(process.Wait()).Should(Receive(Equal(disaster)))
							})

							It("invokes the delegate's Failed callback", func() {
								Eventually(process.Wait()).Should(Receive(Equal(disaster)))
								Expect(taskDelegate.FailedCallCount()).To(Equal(1))
								Expect(taskDelegate.FailedArgsForCall(0)).To(Equal(disaster))
							})
						})
					})

					Context("when creating the container fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeWorker.CreateContainerReturns(nil, disaster)
						})

						It("exits with the error", func() {
							Eventually(process.Wait()).Should(Receive(Equal(disaster)))
						})

						It("invokes the delegate's Failed callback", func() {
							Eventually(process.Wait()).Should(Receive(Equal(disaster)))
							Expect(taskDelegate.FailedCallCount()).To(Equal(1))
							Expect(taskDelegate.FailedArgsForCall(0)).To(Equal(disaster))
						})
					})
				})
			})

			Context("when getting the config fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					configSource.FetchConfigReturns(atc.TaskConfig{}, disaster)
				})

				It("exits with the failure", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
				})

				It("invokes the delegate's Failed callback", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
					Expect(taskDelegate.FailedCallCount()).To(Equal(1))
					Expect(taskDelegate.FailedArgsForCall(0)).To(Equal(disaster))
				})
			})
		})

		Context("when the container already exists", func() {
			var fakeContainer *wfakes.FakeContainer

			BeforeEach(func() {
				fakeContainer = new(wfakes.FakeContainer)
				fakeWorkerClient.FindContainerForIdentifierReturns(fakeContainer, true, nil)
			})

			Context("when an exit status is already saved off", func() {
				BeforeEach(func() {
					fakeContainer.PropertyStub = func(name string) (string, error) {
						defer GinkgoRecover()

						switch name {
						case "concourse:exit-status":
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
					Expect(fakeContainer.AttachCallCount()).To(BeZero())
				})

				It("is not successful", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))

					var success Success
					Expect(step.Result(&success)).To(BeTrue())
					Expect(bool(success)).To(BeFalse())
				})

				It("reports its exit status", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))

					var status ExitStatus
					Expect(step.Result(&status)).To(BeTrue())
					Expect(status).To(Equal(ExitStatus(123)))
				})

				It("does not invoke the delegate's Started callback", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))
					Expect(taskDelegate.StartedCallCount()).To(BeZero())
				})

				It("does not invoke the delegate's Finished callback", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))
					Expect(taskDelegate.FinishedCallCount()).To(BeZero())
				})
			})

			Context("when the process id can be found", func() {
				BeforeEach(func() {
					fakeContainer.PropertyStub = func(name string) (string, error) {
						defer GinkgoRecover()

						switch name {
						case "concourse:task-process":
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
						Expect(fakeContainer.AttachCallCount()).To(Equal(1))

						pid, _ := fakeContainer.AttachArgsForCall(0)
						Expect(pid).To(Equal(uint32(42)))
					})

					It("directs the process's stdout/stderr to the io config", func() {
						Expect(fakeContainer.AttachCallCount()).To(Equal(1))

						_, pio := fakeContainer.AttachArgsForCall(0)
						Expect(pio.Stdout).To(Equal(stdoutBuf))
						Expect(pio.Stderr).To(Equal(stderrBuf))
					})

					It("does not invoke the delegate's Started callback", func() {
						Expect(taskDelegate.StartedCallCount()).To(BeZero())
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
						Expect(taskDelegate.FailedCallCount()).To(Equal(1))
						Expect(taskDelegate.FailedArgsForCall(0)).To(Equal(disaster))
					})
				})
			})

			Context("when the process id cannot be found", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeContainer.PropertyReturns("", disaster)
				})

				It("exits with the failure", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
				})

				It("invokes the delegate's Failed callback", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
					Eventually(taskDelegate.FailedCallCount()).Should(Equal(1))
					Expect(taskDelegate.FailedArgsForCall(0)).To(Equal(disaster))
				})
			})
		})
	})
})
