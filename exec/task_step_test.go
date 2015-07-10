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
	rfakes "github.com/concourse/atc/resource/fakes"
	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("GardenFactory", func() {
	var (
		fakeTracker      *rfakes.FakeTracker
		fakeWorkerClient *wfakes.FakeClient

		factory Factory

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		sourceName SourceName = "some-source-name"

		identifier = worker.Identifier{
			Name: "some-session-id",
		}
	)

	BeforeEach(func() {
		fakeTracker = new(rfakes.FakeTracker)
		fakeWorkerClient = new(wfakes.FakeClient)

		factory = NewGardenFactory(fakeWorkerClient, fakeTracker, func() string {
			return "a-random-guid"
		})

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
			step = factory.Task(sourceName, identifier, taskDelegate, privileged, tags, configSource).Using(inStep, repo)
			process = ifrit.Invoke(step)
		})

		Context("when the container does not yet exist", func() {
			BeforeEach(func() {
				fakeWorkerClient.LookupContainerReturns(nil, errors.New("nope"))
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

				Context("when creating the task's container works", func() {
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
							taskDelegate.InitializingStub = func(atc.TaskConfig) {
								defer GinkgoRecover()
								Ω(fakeWorkerClient.CreateContainerCallCount()).Should(BeZero())
							}
						})

						It("invokes the delegate's Initializing callback", func() {
							Ω(taskDelegate.InitializingCallCount()).Should(Equal(1))
							Ω(taskDelegate.InitializingArgsForCall(0)).Should(Equal(fetchedConfig))
						})
					})

					It("looked up the container via the session ID", func() {
						Ω(fakeWorkerClient.LookupContainerArgsForCall(0)).Should(Equal(identifier))
					})

					It("gets the config from the input artifact soruce", func() {
						Ω(configSource.FetchConfigCallCount()).Should(Equal(1))
						Ω(configSource.FetchConfigArgsForCall(0)).Should(Equal(repo))
					})

					It("creates a container with the config's image and the session ID as the handle", func() {
						Ω(fakeWorkerClient.CreateContainerCallCount()).Should(Equal(1))
						createdIdentifier, spec := fakeWorkerClient.CreateContainerArgsForCall(0)
						Ω(createdIdentifier).Should(Equal(identifier))

						taskSpec := spec.(worker.TaskContainerSpec)
						Ω(taskSpec.Platform).Should(Equal("some-platform"))
						Ω(taskSpec.Tags).Should(ConsistOf("config", "step", "tags"))
						Ω(taskSpec.Image).Should(Equal("some-image"))
						Ω(taskSpec.Privileged).Should(BeFalse())

					})

					It("ensures artifacts root exists by streaming in an empty payload", func() {
						Ω(fakeContainer.StreamInCallCount()).Should(Equal(1))

						spec := fakeContainer.StreamInArgsForCall(0)
						Ω(spec.Path).Should(Equal("/tmp/build/a-random-guid"))
						Ω(spec.User).Should(Equal("root"))

						tarReader := tar.NewReader(spec.TarStream)

						_, err := tarReader.Next()
						Ω(err).Should(Equal(io.EOF))
					})

					It("runs a process with the config's path and args, in the specified build directory", func() {
						Ω(fakeContainer.RunCallCount()).Should(Equal(1))

						spec, _ := fakeContainer.RunArgsForCall(0)
						Ω(spec.Path).Should(Equal("ls"))
						Ω(spec.Args).Should(Equal([]string{"some", "args"}))
						Ω(spec.Env).Should(Equal([]string{"SOME=params"}))
						Ω(spec.Dir).Should(Equal("/tmp/build/a-random-guid"))
						Ω(spec.User).Should(Equal("root"))
						Ω(spec.TTY).Should(Equal(&garden.TTYSpec{}))
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
						Ω(name).Should(Equal("concourse:task-process"))
						Ω(value).Should(Equal("42"))
					})

					It("invokes the delegate's Started callback", func() {
						Ω(taskDelegate.StartedCallCount()).Should(Equal(1))
					})

					Context("when privileged", func() {
						BeforeEach(func() {
							privileged = true
						})

						It("creates the container privileged", func() {
							Ω(fakeWorkerClient.CreateContainerCallCount()).Should(Equal(1))
							createdIdentifier, spec := fakeWorkerClient.CreateContainerArgsForCall(0)
							Ω(createdIdentifier).Should(Equal(identifier))

							taskSpec := spec.(worker.TaskContainerSpec)
							Ω(taskSpec.Platform).Should(Equal("some-platform"))
							Ω(taskSpec.Tags).Should(ConsistOf("config", "step", "tags"))
							Ω(taskSpec.Image).Should(Equal("some-image"))
							Ω(taskSpec.Privileged).Should(BeTrue())
						})

						It("runs the process as root", func() {
							Ω(fakeContainer.RunCallCount()).Should(Equal(1))

							spec, _ := fakeContainer.RunArgsForCall(0)
							Ω(spec).Should(Equal(garden.ProcessSpec{
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
								Image:  "some-image",
								Params: map[string]string{"SOME": "params"},
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

								Ω(inputSource.StreamToCallCount()).Should(Equal(1))

								destination := inputSource.StreamToArgsForCall(0)

								initial := fakeContainer.StreamInCallCount()

								err := destination.StreamIn("foo", streamIn)
								Ω(err).ShouldNot(HaveOccurred())

								Ω(fakeContainer.StreamInCallCount()).Should(Equal(initial + 1))

								spec := fakeContainer.StreamInArgsForCall(initial)
								Ω(spec.Path).Should(Equal("/tmp/build/a-random-guid/some-input-configured-path/foo"))
								Ω(spec.User).Should(Equal("root"))
								Ω(spec.TarStream).Should(Equal(streamIn))

								Ω(otherInputSource.StreamToCallCount()).Should(Equal(1))

								destination = otherInputSource.StreamToArgsForCall(0)

								initial = fakeContainer.StreamInCallCount()

								err = destination.StreamIn("foo", streamIn)
								Ω(err).ShouldNot(HaveOccurred())

								Ω(fakeContainer.StreamInCallCount()).Should(Equal(initial + 1))
								spec = fakeContainer.StreamInArgsForCall(initial)
								Ω(spec.Path).Should(Equal("/tmp/build/a-random-guid/some-other-input/foo"))
								Ω(spec.User).Should(Equal("root"))
								Ω(spec.TarStream).Should(Equal(streamIn))

								Eventually(process.Wait()).Should(Receive(BeNil()))
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
									Ω(fakeContainer.RunCallCount()).Should(Equal(0))
								})

								It("invokes the delegate's Failed callback", func() {
									Eventually(process.Wait()).Should(Receive(Equal(disaster)))
									Ω(taskDelegate.FailedCallCount()).Should(Equal(1))
									Ω(taskDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
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
								Ω(err).Should(BeAssignableToTypeOf(MissingInputsError{}))
								Ω(err.(MissingInputsError).Inputs).Should(ConsistOf("some-other-input"))
							})

							It("invokes the delegate's Failed callback", func() {
								Eventually(process.Wait()).Should(Receive(HaveOccurred()))

								Ω(taskDelegate.FailedCallCount()).Should(Equal(1))

								err := taskDelegate.FailedArgsForCall(0)
								Ω(err).Should(BeAssignableToTypeOf(MissingInputsError{}))
								Ω(err.(MissingInputsError).Inputs).Should(ConsistOf("some-other-input"))
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
							Ω(name).Should(Equal("concourse:exit-status"))
							Ω(value).Should(Equal("0"))
						})

						It("is successful", func() {
							Eventually(process.Wait()).Should(Receive(BeNil()))

							var success Success
							Ω(step.Result(&success)).Should(BeTrue())
							Ω(bool(success)).Should(BeTrue())
						})

						It("reports its exit status", func() {
							Eventually(process.Wait()).Should(Receive(BeNil()))

							var status ExitStatus
							Ω(step.Result(&status)).Should(BeTrue())
							Ω(status).Should(Equal(ExitStatus(0)))
						})

						Describe("the registered source", func() {
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
									var streamedOut io.ReadCloser

									BeforeEach(func() {
										streamedOut = gbytes.NewBuffer()
										fakeContainer.StreamOutReturns(streamedOut, nil)
									})

									It("streams the resource to the destination", func() {
										err := artifactSource.StreamTo(fakeDestination)
										Ω(err).ShouldNot(HaveOccurred())

										Ω(fakeContainer.StreamOutCallCount()).Should(Equal(1))
										spec := fakeContainer.StreamOutArgsForCall(0)
										Ω(spec.Path).Should(Equal("/tmp/build/a-random-guid/"))
										Ω(spec.User).Should(Equal("root"))

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

								Context("when the container cannot stream out", func() {
									disaster := errors.New("nope")

									BeforeEach(func() {
										fakeContainer.StreamOutReturns(nil, disaster)
									})

									It("returns the error", func() {
										Ω(artifactSource.StreamTo(fakeDestination)).Should(Equal(disaster))
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
											reader, err := artifactSource.StreamFile("some-path")
											Ω(err).ShouldNot(HaveOccurred())

											Ω(ioutil.ReadAll(reader)).Should(Equal([]byte(fileContent)))

											spec := fakeContainer.StreamOutArgsForCall(0)
											Ω(spec.Path).Should(Equal("/tmp/build/a-random-guid/some-path"))
											Ω(spec.User).Should(Equal("root"))
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

								Context("when the container cannot stream out", func() {
									disaster := errors.New("nope")

									BeforeEach(func() {
										fakeContainer.StreamOutReturns(nil, disaster)
									})

									It("returns the error", func() {
										_, err := artifactSource.StreamFile("some-path")
										Ω(err).Should(Equal(disaster))
									})
								})
							})
						})

						Describe("before saving the exit status property", func() {
							BeforeEach(func() {
								taskDelegate.FinishedStub = func(ExitStatus) {
									callCount := fakeContainer.SetPropertyCallCount()

									for i := 0; i < callCount; i++ {
										name, _ := fakeContainer.SetPropertyArgsForCall(i)
										Ω(name).ShouldNot(Equal("concourse:exit-status"))
									}
								}
							})

							It("invokes the delegate's Finished callback", func() {
								Eventually(process.Wait()).Should(Receive(BeNil()))

								Ω(taskDelegate.FinishedCallCount()).Should(Equal(1))
								Ω(taskDelegate.FinishedArgsForCall(0)).Should(Equal(ExitStatus(0)))
							})
						})

						Context("when saving the exit status fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeContainer.SetPropertyStub = func(name string, value string) error {
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
								Ω(taskDelegate.FailedCallCount()).Should(Equal(1))
								Ω(taskDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
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
							Ω(name).Should(Equal("concourse:exit-status"))
							Ω(value).Should(Equal("1"))
						})

						It("is not successful", func() {
							Eventually(process.Wait()).Should(Receive(BeNil()))

							var success Success
							Ω(step.Result(&success)).Should(BeTrue())
							Ω(bool(success)).Should(BeFalse())
						})

						It("reports its exit status", func() {
							Eventually(process.Wait()).Should(Receive(BeNil()))

							var status ExitStatus
							Ω(step.Result(&status)).Should(BeTrue())
							Ω(status).Should(Equal(ExitStatus(1)))
						})

						Describe("before saving the exit status property", func() {
							BeforeEach(func() {
								taskDelegate.FinishedStub = func(ExitStatus) {
									callCount := fakeContainer.SetPropertyCallCount()

									for i := 0; i < callCount; i++ {
										name, _ := fakeContainer.SetPropertyArgsForCall(i)
										Ω(name).ShouldNot(Equal("concourse:exit-status"))
									}
								}
							})

							It("invokes the delegate's Finished callback", func() {
								Eventually(process.Wait()).Should(Receive(BeNil()))

								Ω(taskDelegate.FinishedCallCount()).Should(Equal(1))
								Ω(taskDelegate.FinishedArgsForCall(0)).Should(Equal(ExitStatus(1)))
							})
						})

						Context("when saving the exit status fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeContainer.SetPropertyStub = func(name string, value string) error {
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
								Ω(taskDelegate.FailedCallCount()).Should(Equal(1))
								Ω(taskDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
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
							Ω(taskDelegate.FailedCallCount()).Should(Equal(1))
							Ω(taskDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
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
							Ω(taskDelegate.FailedCallCount()).Should(Equal(1))
							Ω(taskDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
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

							err := step.Release()
							Ω(err).ShouldNot(HaveOccurred())

							Ω(fakeContainer.ReleaseCallCount()).Should(Equal(1))
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
							Ω(taskDelegate.FailedCallCount()).Should(Equal(1))
							Ω(taskDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
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
						Ω(taskDelegate.FailedCallCount()).Should(Equal(1))
						Ω(taskDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
					})

					Describe("releasing", func() {
						JustBeforeEach(func() {
							Eventually(process.Wait()).Should(Receive(Equal(disaster)))
						})

						It("succeeds", func() {
							err := step.Release()
							Ω(err).ShouldNot(HaveOccurred())
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
					Ω(taskDelegate.FailedCallCount()).Should(Equal(1))
					Ω(taskDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
				})
			})
		})

		Context("when the container already exists", func() {
			var fakeContainer *wfakes.FakeContainer

			BeforeEach(func() {
				fakeContainer = new(wfakes.FakeContainer)
				fakeWorkerClient.LookupContainerReturns(fakeContainer, nil)
			})

			Context("when an exit status is already saved off", func() {
				BeforeEach(func() {
					fakeContainer.PropertyStub = func(name string) (string, error) {
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
					Ω(fakeContainer.AttachCallCount()).Should(BeZero())
				})

				It("is not successful", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))

					var success Success
					Ω(step.Result(&success)).Should(BeTrue())
					Ω(bool(success)).Should(BeFalse())
				})

				It("reports its exit status", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))

					var status ExitStatus
					Ω(step.Result(&status)).Should(BeTrue())
					Ω(status).Should(Equal(ExitStatus(123)))
				})

				It("does not invoke the delegate's Started callback", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))
					Ω(taskDelegate.StartedCallCount()).Should(BeZero())
				})

				It("does not invoke the delegate's Finished callback", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))
					Ω(taskDelegate.FinishedCallCount()).Should(BeZero())
				})

				It("invokes the delegate's Result callback", func() {
					// If another build plan is resumed and sees that a task has already
					// run it should restore the task's original exit status rather than
					// just assuming everything was a-ok. Otherwise the plan is not
					// re-entrant, idempotent, and (most importantly) not correct.
					//
					// Calling Result() will make sure that no false assumptions are made
					// but it doesn't save a dupicate build event.

					Eventually(process.Wait()).Should(Receive(BeNil()))
					Ω(taskDelegate.ResultCallCount()).Should(Equal(1))
					Ω(taskDelegate.ResultArgsForCall(0)).Should(Equal(ExitStatus(123)))
				})
			})

			Context("when the process id can be found", func() {
				BeforeEach(func() {
					fakeContainer.PropertyStub = func(name string) (string, error) {
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
						Ω(taskDelegate.StartedCallCount()).Should(BeZero())
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
						Ω(taskDelegate.FailedCallCount()).Should(Equal(1))
						Ω(taskDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
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
					Ω(taskDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
				})
			})
		})
	})
})
