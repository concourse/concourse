package exec_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

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
	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("GardenFactory", func() {
	var (
		fakeWorkerClient *wfakes.FakeClient
		fakeTracker      *rfakes.FakeTracker

		factory Factory

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer
		fakeClock *fakeclock.FakeClock

		sourceName        SourceName = "some-source-name"
		imageArtifactName string
		identifier        = worker.Identifier{
			BuildID: 1234,
			PlanID:  atc.PlanID("some-plan-id"),
		}
		workerMetadata = worker.Metadata{
			PipelineName: "some-pipeline",
			Type:         db.ContainerTypeTask,
			StepName:     "some-step",
		}
	)

	BeforeEach(func() {
		fakeWorkerClient = new(wfakes.FakeClient)
		fakeTracker = new(rfakes.FakeTracker)

		factory = NewGardenFactory(fakeWorkerClient, fakeTracker)

		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
	})

	Describe("Task", func() {
		var (
			taskDelegate  *fakes.FakeTaskDelegate
			privileged    Privileged
			tags          []string
			configSource  *fakes.FakeTaskConfigSource
			resourceTypes atc.ResourceTypes
			inputMapping  map[string]string
			outputMapping map[string]string

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

			resourceTypes = atc.ResourceTypes{
				{
					Name:   "custom-resource",
					Type:   "custom-type",
					Source: atc.Source{"some-custom": "source"},
				},
			}

			inputMapping = nil
			outputMapping = nil
			imageArtifactName = ""
			fakeClock = fakeclock.NewFakeClock(time.Unix(0, 123))
		})

		JustBeforeEach(func() {
			step = factory.Task(
				lagertest.NewTestLogger("test"),
				sourceName,
				identifier,
				workerMetadata,
				taskDelegate,
				privileged,
				tags,
				configSource,
				resourceTypes,
				inputMapping,
				outputMapping,
				imageArtifactName,
				fakeClock,
			).Using(inStep, repo)

			process = ifrit.Invoke(step)
		})

		Context("when the container does not yet exist", func() {
			BeforeEach(func() {
				fakeWorkerClient.FindContainerForIdentifierReturns(nil, false, errors.New("nope"))
			})

			Context("when getting the config works", func() {
				var fetchedConfig atc.TaskConfig

				BeforeEach(func() {
					fetchedConfig = atc.TaskConfig{
						Platform: "some-platform",
						Image:    "some-image",
						ImageResource: &atc.ImageResource{
							Type:   "docker",
							Source: atc.Source{"some": "source"},
						},
						Params: map[string]string{"SOME": "params"},
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
						fakeWorkerClient.AllSatisfyingReturns(nil, disaster)
					})

					It("exits with the error", func() {
						Expect(<-process.Wait()).To(Equal(disaster))
					})
				})

				Context("when a single worker can be located", func() {
					var fakeWorker *wfakes.FakeWorker

					BeforeEach(func() {
						fakeWorker = new(wfakes.FakeWorker)
						fakeWorkerClient.AllSatisfyingReturns([]worker.Worker{fakeWorker}, nil)
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
							fakeProcess.IDReturns("process-id")
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

						It("found the worker with the right spec", func() {
							Expect(fakeWorkerClient.AllSatisfyingCallCount()).To(Equal(1))
							spec, actualResourceTypes := fakeWorkerClient.AllSatisfyingArgsForCall(0)
							Expect(spec.Platform).To(Equal("some-platform"))
							Expect(actualResourceTypes).To(Equal(atc.ResourceTypes{
								{
									Name:   "custom-resource",
									Type:   "custom-type",
									Source: atc.Source{"some-custom": "source"},
								},
							}))
						})

						It("looked up the container via the session ID across the entire pool", func() {
							_, findID := fakeWorkerClient.FindContainerForIdentifierArgsForCall(0)
							Expect(findID).To(Equal(worker.Identifier{
								BuildID: 1234,
								PlanID:  atc.PlanID("some-plan-id"),
								Stage:   db.ContainerStageRun,
							}))
						})

						It("gets the config from the input artifact source", func() {
							Expect(configSource.FetchConfigCallCount()).To(Equal(1))
							Expect(configSource.FetchConfigArgsForCall(0)).To(Equal(repo))
						})

						It("creates a container with the config's image and the session ID as the handle", func() {
							Expect(fakeWorker.CreateContainerCallCount()).To(Equal(1))
							_, _, delegate, createdIdentifier, createdMetadata, spec, actualResourceTypes := fakeWorker.CreateContainerArgsForCall(0)
							Expect(createdIdentifier).To(Equal(worker.Identifier{
								BuildID: 1234,
								PlanID:  atc.PlanID("some-plan-id"),
								Stage:   db.ContainerStageRun,
							}))
							Expect(createdMetadata).To(Equal(worker.Metadata{
								PipelineName:         "some-pipeline",
								Type:                 db.ContainerTypeTask,
								StepName:             "some-step",
								WorkingDirectory:     "/tmp/build/a1f5c0c1",
								EnvironmentVariables: []string{"SOME=params"},
							}))

							Expect(delegate).To(Equal(taskDelegate))

							Expect(spec.Platform).To(Equal("some-platform"))
							Expect(spec.ImageSpec).To(Equal(worker.ImageSpec{
								ImageURL: "some-image",
								ImageResource: &atc.ImageResource{
									Type:   "docker",
									Source: atc.Source{"some": "source"},
								},
								Privileged: false,
							}))

							Expect(actualResourceTypes).To(Equal(atc.ResourceTypes{
								{
									Name:   "custom-resource",
									Type:   "custom-type",
									Source: atc.Source{"some-custom": "source"},
								},
							}))
						})

						It("ensures artifacts root exists by streaming in an empty payload", func() {
							Expect(fakeContainer.StreamInCallCount()).To(Equal(1))

							spec := fakeContainer.StreamInArgsForCall(0)
							Expect(spec.Path).To(Equal("/tmp/build/a1f5c0c1"))
							Expect(spec.User).To(Equal("")) // use default

							tarReader := tar.NewReader(spec.TarStream)

							_, err := tarReader.Next()
							Expect(err).To(Equal(io.EOF))
						})

						It("runs a process with the config's path and args, in the specified (default) build directory", func() {
							Expect(fakeContainer.RunCallCount()).To(Equal(1))

							spec, _ := fakeContainer.RunArgsForCall(0)
							Expect(spec.Path).To(Equal("ls"))
							Expect(spec.Args).To(Equal([]string{"some", "args"}))
							Expect(spec.Env).To(Equal([]string{"SOME=params"}))
							Expect(spec.Dir).To(Equal("/tmp/build/a1f5c0c1"))
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
							Expect(value).To(Equal("process-id"))
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
								_, _, _, createdIdentifier, createdMetadata, spec, _ := fakeWorker.CreateContainerArgsForCall(0)
								Expect(createdIdentifier).To(Equal(worker.Identifier{
									BuildID: 1234,
									PlanID:  atc.PlanID("some-plan-id"),
									Stage:   db.ContainerStageRun,
								}))
								Expect(createdMetadata).To(Equal(worker.Metadata{
									PipelineName:         "some-pipeline",
									Type:                 db.ContainerTypeTask,
									StepName:             "some-step",
									WorkingDirectory:     "/tmp/build/a1f5c0c1",
									EnvironmentVariables: []string{"SOME=params"},
								}))

								Expect(spec.Platform).To(Equal("some-platform"))
								Expect(spec.ImageSpec).To(Equal(worker.ImageSpec{
									ImageURL: "some-image",
									ImageResource: &atc.ImageResource{
										Type:   "docker",
										Source: atc.Source{"some": "source"},
									},
									Privileged: true,
								}))
							})

							It("runs the process as the specified user", func() {
								Expect(fakeContainer.RunCallCount()).To(Equal(1))

								spec, _ := fakeContainer.RunArgsForCall(0)
								Expect(spec).To(Equal(garden.ProcessSpec{
									Path: "ls",
									Args: []string{"some", "args"},
									Env:  []string{"SOME=params"},
									Dir:  "/tmp/build/a1f5c0c1",
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
									Expect(spec.Path).To(Equal("/tmp/build/a1f5c0c1/some-input-configured-path/foo"))
									Expect(spec.User).To(Equal("")) // use default
									Expect(spec.TarStream).To(Equal(streamIn))

									Expect(otherInputSource.StreamToCallCount()).To(Equal(1))

									destination = otherInputSource.StreamToArgsForCall(0)

									initial = fakeContainer.StreamInCallCount()

									err = destination.StreamIn("foo", streamIn)
									Expect(err).NotTo(HaveOccurred())

									Expect(fakeContainer.StreamInCallCount()).To(Equal(initial + 1))
									spec = fakeContainer.StreamInArgsForCall(initial)
									Expect(spec.Path).To(Equal("/tmp/build/a1f5c0c1/some-other-input/foo"))
									Expect(spec.User).To(Equal("")) // use default
									Expect(spec.TarStream).To(Equal(streamIn))

									Eventually(process.Wait()).Should(Receive(BeNil()))
								})

								Context("when the inputs have volumes on the chosen worker", func() {
									var inputVolume *wfakes.FakeVolume
									var otherInputVolume *wfakes.FakeVolume

									BeforeEach(func() {
										inputVolume = new(wfakes.FakeVolume)
										inputVolume.HandleReturns("input-volume")

										otherInputVolume = new(wfakes.FakeVolume)
										otherInputVolume.HandleReturns("other-input-volume")

										inputSource.VolumeOnReturns(inputVolume, true, nil)
										otherInputSource.VolumeOnReturns(otherInputVolume, true, nil)
									})

									It("bind-mounts copy-on-write volumes to their destinations in the container", func() {
										_, _, _, _, _, spec, _ := fakeWorker.CreateContainerArgsForCall(0)
										Expect(spec.Inputs).To(Equal([]worker.VolumeMount{
											{
												Volume:    inputVolume,
												MountPath: "/tmp/build/a1f5c0c1/some-input-configured-path",
											},
											{
												Volume:    otherInputVolume,
												MountPath: "/tmp/build/a1f5c0c1/some-other-input",
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

						Context("when input is remapped", func() {
							var remappedInputSource *fakes.FakeArtifactSource

							BeforeEach(func() {
								remappedInputSource = new(fakes.FakeArtifactSource)
								inputMapping = map[string]string{"remapped-input": "remapped-input-src"}

								configSource.FetchConfigReturns(atc.TaskConfig{
									Run: atc.TaskRunConfig{
										Path: "ls",
									},
									Inputs: []atc.TaskInputConfig{
										{Name: "remapped-input"},
									},
								}, nil)
							})

							Context("when all inputs are present in the in source repository", func() {
								BeforeEach(func() {
									repo.RegisterSource("remapped-input-src", remappedInputSource)
								})

								It("uses remapped input", func() {
									streamIn := new(bytes.Buffer)

									Expect(remappedInputSource.StreamToCallCount()).To(Equal(1))

									destination := remappedInputSource.StreamToArgsForCall(0)

									initial := fakeContainer.StreamInCallCount()

									err := destination.StreamIn("foo", streamIn)
									Expect(err).NotTo(HaveOccurred())

									Expect(fakeContainer.StreamInCallCount()).To(Equal(initial + 1))

									spec := fakeContainer.StreamInArgsForCall(initial)
									Expect(spec.Path).To(Equal("/tmp/build/a1f5c0c1/remapped-input/foo"))
									Expect(spec.User).To(Equal("")) // use default
									Expect(spec.TarStream).To(Equal(streamIn))

									Eventually(process.Wait()).Should(Receive(BeNil()))
								})

								Context("when the inputs have volumes on the chosen worker", func() {
									var remappedInputVolume *wfakes.FakeVolume

									BeforeEach(func() {
										remappedInputVolume = new(wfakes.FakeVolume)
										remappedInputVolume.HandleReturns("remapped-input-volume")

										remappedInputSource.VolumeOnReturns(remappedInputVolume, true, nil)
									})

									It("bind-mounts copy-on-write volumes to their destinations in the container", func() {
										_, _, _, _, _, spec, _ := fakeWorker.CreateContainerArgsForCall(0)
										Expect(spec.Inputs).To(Equal([]worker.VolumeMount{
											{
												Volume:    remappedInputVolume,
												MountPath: "/tmp/build/a1f5c0c1/remapped-input",
											},
										}))
									})

									It("releases the volumes given to the worker", func() {
										Expect(remappedInputVolume.ReleaseCallCount()).To(Equal(1))
									})

									It("does not stream inputs that had volumes", func() {
										Expect(remappedInputSource.StreamToCallCount()).To(Equal(0))
									})
								})
							})

							Context("when any of the inputs are missing", func() {
								It("exits with failure", func() {
									var err error
									Eventually(process.Wait()).Should(Receive(&err))
									Expect(err).To(BeAssignableToTypeOf(MissingInputsError{}))
									Expect(err.(MissingInputsError).Inputs).To(ConsistOf("remapped-input-src"))
								})

								It("invokes the delegate's Failed callback", func() {
									Eventually(process.Wait()).Should(Receive(HaveOccurred()))

									Expect(taskDelegate.FailedCallCount()).To(Equal(1))

									err := taskDelegate.FailedArgsForCall(0)
									Expect(err).To(BeAssignableToTypeOf(MissingInputsError{}))
									Expect(err.(MissingInputsError).Inputs).To(ConsistOf("remapped-input-src"))
								})
							})
						})

						Context("when the configuration specifies paths for outputs", func() {
							BeforeEach(func() {
								configSource.FetchConfigReturns(atc.TaskConfig{
									Platform: "some-platform",
									Image:    "some-image",
									Params:   map[string]string{"SOME": "params"},
									Run: atc.TaskRunConfig{
										Path: "ls",
										Args: []string{"some", "args"},
									},
									Outputs: []atc.TaskOutputConfig{
										{Name: "some-output", Path: "some-output-configured-path"},
										{Name: "some-other-output"},
										{Name: "some-trailing-slash-output", Path: "some-output-configured-path-with-trailing-slash/"},
									},
								}, nil)

								fakeWorker.CreateVolumeReturns(new(wfakes.FakeVolume), nil)
							})

							It("ensures the output directories exist by streaming in an empty payload", func() {
								Expect(fakeContainer.StreamInCallCount()).To(Equal(4))

								spec := fakeContainer.StreamInArgsForCall(1)
								Expect(spec.Path).To(Equal("/tmp/build/a1f5c0c1/some-output-configured-path/"))
								Expect(spec.User).To(Equal("")) // use default

								tarReader := tar.NewReader(spec.TarStream)

								_, err := tarReader.Next()
								Expect(err).To(Equal(io.EOF))

								spec = fakeContainer.StreamInArgsForCall(2)
								Expect(spec.Path).To(Equal("/tmp/build/a1f5c0c1/some-other-output/"))
								Expect(spec.User).To(Equal("")) // use default

								tarReader = tar.NewReader(spec.TarStream)

								_, err = tarReader.Next()
								Expect(err).To(Equal(io.EOF))

								spec = fakeContainer.StreamInArgsForCall(3)
								Expect(spec.Path).To(Equal("/tmp/build/a1f5c0c1/some-output-configured-path-with-trailing-slash/"))
								Expect(spec.User).To(Equal("")) // use default

								tarReader = tar.NewReader(spec.TarStream)

								_, err = tarReader.Next()
								Expect(err).To(Equal(io.EOF))
							})

							Context("when the process exits 0", func() {
								BeforeEach(func() {
									fakeProcess.WaitReturns(0, nil)
								})

								Describe("the registered sources", func() {
									var (
										artifactSource1 ArtifactSource
										artifactSource2 ArtifactSource
										artifactSource3 ArtifactSource

										fakeMountPath1 string = "/tmp/build/a1f5c0c1/some-output-configured-path/"
										fakeMountPath2 string = "/tmp/build/a1f5c0c1/some-other-output/"
										fakeMountPath3 string = "/tmp/build/a1f5c0c1/some-output-configured-path-with-trailing-slash/"
									)

									JustBeforeEach(func() {
										Eventually(process.Wait()).Should(Receive(BeNil()))

										var found bool
										artifactSource1, found = repo.SourceFor("some-output")
										Expect(found).To(BeTrue())

										artifactSource2, found = repo.SourceFor("some-other-output")
										Expect(found).To(BeTrue())

										artifactSource3, found = repo.SourceFor("some-trailing-slash-output")
										Expect(found).To(BeTrue())
									})

									It("does not register the task as a source", func() {
										sourceMap := repo.AsMap()
										Expect(sourceMap).To(ConsistOf(artifactSource1, artifactSource2, artifactSource3))
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

											Context("when volumes are configured", func() {
												var (
													fakeNewlyCreatedVolume1 *wfakes.FakeVolume
													fakeNewlyCreatedVolume2 *wfakes.FakeVolume
													fakeNewlyCreatedVolume3 *wfakes.FakeVolume

													fakeVolume1 *wfakes.FakeVolume
													fakeVolume2 *wfakes.FakeVolume
													fakeVolume3 *wfakes.FakeVolume
												)

												BeforeEach(func() {
													fakeNewlyCreatedVolume1 = new(wfakes.FakeVolume)
													fakeNewlyCreatedVolume1.HandleReturns("some-handle-1")
													fakeNewlyCreatedVolume2 = new(wfakes.FakeVolume)
													fakeNewlyCreatedVolume2.HandleReturns("some-handle-2")
													fakeNewlyCreatedVolume3 = new(wfakes.FakeVolume)
													fakeNewlyCreatedVolume3.HandleReturns("some-handle-3")
													volumeChannel := make(chan worker.Volume, 3)
													volumeChannel <- fakeNewlyCreatedVolume1
													volumeChannel <- fakeNewlyCreatedVolume2
													volumeChannel <- fakeNewlyCreatedVolume3
													close(volumeChannel)

													fakeWorker.CreateVolumeStub = func(lager.Logger, worker.VolumeSpec) (worker.Volume, error) {
														return <-volumeChannel, nil
													}

													fakeVolume1 = new(wfakes.FakeVolume)
													fakeVolume1.HandleReturns("some-handle-1")
													fakeVolume2 = new(wfakes.FakeVolume)
													fakeVolume2.HandleReturns("some-handle-2")
													fakeVolume3 = new(wfakes.FakeVolume)
													fakeVolume3.HandleReturns("some-handle-3")

													fakeContainer.VolumeMountsReturns([]worker.VolumeMount{
														worker.VolumeMount{
															Volume:    fakeVolume1,
															MountPath: fakeMountPath1,
														},
														worker.VolumeMount{
															Volume:    fakeVolume2,
															MountPath: fakeMountPath2,
														},
														worker.VolumeMount{
															Volume:    fakeVolume3,
															MountPath: fakeMountPath3,
														},
													})

													fakeWorker.NameReturns("bananapants")
												})

												It("creates volumes for each output", func() {
													Expect(fakeWorker.CreateVolumeCallCount()).To(Equal(3))

													_, vSpec := fakeWorker.CreateVolumeArgsForCall(0)
													Expect(vSpec).To(Equal(worker.VolumeSpec{
														Strategy: worker.OutputStrategy{
															Name: "some-output",
														},
														TTL:        worker.VolumeTTL,
														Privileged: bool(privileged),
													}))

													_, vSpec = fakeWorker.CreateVolumeArgsForCall(1)
													Expect(vSpec).To(Equal(worker.VolumeSpec{
														Strategy: worker.OutputStrategy{
															Name: "some-other-output",
														},
														TTL:        worker.VolumeTTL,
														Privileged: bool(privileged),
													}))

													_, vSpec = fakeWorker.CreateVolumeArgsForCall(2)
													Expect(vSpec).To(Equal(worker.VolumeSpec{
														Strategy: worker.OutputStrategy{
															Name: "some-trailing-slash-output",
														},
														TTL:        worker.VolumeTTL,
														Privileged: bool(privileged),
													}))
												})

												It("passes the created output volumes to the worker", func() {
													_, _, _, _, _, spec, _ := fakeWorker.CreateContainerArgsForCall(0)
													var actualVolumes []worker.Volume
													var actualPaths []string
													for _, v := range spec.Outputs {
														actualVolume, ok := v.Volume.(worker.Volume)
														Expect(ok).To(BeTrue())
														actualVolumes = append(actualVolumes, actualVolume)
														actualPaths = append(actualPaths, v.MountPath)
													}

													Expect(actualVolumes).To(ConsistOf(fakeNewlyCreatedVolume1, fakeNewlyCreatedVolume2, fakeNewlyCreatedVolume3))
													Expect(actualPaths).To(ConsistOf(fakeMountPath1, fakeMountPath2, fakeMountPath3))
												})

												It("releases the volumes given to the worker", func() {
													Expect(fakeNewlyCreatedVolume1.ReleaseCallCount()).To(Equal(1))
													Expect(fakeNewlyCreatedVolume2.ReleaseCallCount()).To(Equal(1))
													Expect(fakeNewlyCreatedVolume3.ReleaseCallCount()).To(Equal(1))
												})

												Context("when the output volume can be found on the worker", func() {
													BeforeEach(func() {
														fakeWorker.LookupVolumeReturns(fakeVolume1, true, nil)
													})

													It("stores an artifact source in the repo that can be used to mount the volume", func() {
														actualVolume1, found, err := artifactSource1.VolumeOn(fakeWorker)
														Expect(err).ToNot(HaveOccurred())
														Expect(found).To(BeTrue())
														Expect(actualVolume1).To(Equal(fakeVolume1))

														Expect(fakeWorker.LookupVolumeCallCount()).To(Equal(1))
														_, handle := fakeWorker.LookupVolumeArgsForCall(0)
														Expect(handle).To(Equal("some-handle-1"))
													})
												})

												Context("when the output volume cannot be found on the worker", func() {
													BeforeEach(func() {
														fakeWorker.LookupVolumeReturns(nil, false, nil)
													})

													It("stores an artifact source in the repo that can be used to mount the volume", func() {
														_, found, err := artifactSource1.VolumeOn(fakeWorker)
														Expect(err).ToNot(HaveOccurred())
														Expect(found).To(BeFalse())
													})
												})

												It("streams the data from the volumes to the destination", func() {
													err := artifactSource1.StreamTo(fakeDestination)
													Expect(err).NotTo(HaveOccurred())

													Expect(fakeContainer.StreamOutCallCount()).To(Equal(1))
													spec := fakeContainer.StreamOutArgsForCall(0)
													Expect(spec.Path).To(Equal("/tmp/build/a1f5c0c1/some-output-configured-path/"))
													Expect(spec.User).To(Equal("")) // use default

													Expect(fakeDestination.StreamInCallCount()).To(Equal(1))
													dest, src := fakeDestination.StreamInArgsForCall(0)
													Expect(dest).To(Equal("."))
													Expect(src).To(Equal(streamedOut))
												})

											})

											Context("when volumes are not configured", func() {
												It("streams the configured path to the destination with a trailing slash", func() {
													err := artifactSource1.StreamTo(fakeDestination)
													Expect(err).NotTo(HaveOccurred())

													Expect(fakeContainer.StreamOutCallCount()).To(Equal(1))
													spec := fakeContainer.StreamOutArgsForCall(0)
													Expect(spec.Path).To(Equal("/tmp/build/a1f5c0c1/some-output-configured-path/"))
													Expect(spec.User).To(Equal("")) // use default

													Expect(fakeDestination.StreamInCallCount()).To(Equal(1))
													dest, src := fakeDestination.StreamInArgsForCall(0)
													Expect(dest).To(Equal("."))
													Expect(src).To(Equal(streamedOut))
												})

												It("does not add a redundant trailing slash", func() {
													err := artifactSource3.StreamTo(fakeDestination)
													Expect(err).NotTo(HaveOccurred())

													Expect(fakeContainer.StreamOutCallCount()).To(Equal(1))
													spec := fakeContainer.StreamOutArgsForCall(0)
													Expect(spec.Path).To(Equal("/tmp/build/a1f5c0c1/some-output-configured-path-with-trailing-slash/"))
													Expect(spec.User).To(Equal("")) // use default

													Expect(fakeDestination.StreamInCallCount()).To(Equal(1))
													dest, src := fakeDestination.StreamInArgsForCall(0)
													Expect(dest).To(Equal("."))
													Expect(src).To(Equal(streamedOut))
												})

												It("defaults the path to the output's name", func() {
													err := artifactSource2.StreamTo(fakeDestination)
													Expect(err).NotTo(HaveOccurred())

													Expect(fakeContainer.StreamOutCallCount()).To(Equal(1))
													spec := fakeContainer.StreamOutArgsForCall(0)
													Expect(spec.Path).To(Equal("/tmp/build/a1f5c0c1/some-other-output/"))
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
														Expect(artifactSource1.StreamTo(fakeDestination)).To(Equal(disaster))
													})
												})

												Context("when streaming in to the destination fails", func() {
													disaster := errors.New("nope")

													BeforeEach(func() {
														fakeDestination.StreamInReturns(disaster)
													})

													It("returns the error", func() {
														Expect(artifactSource1.StreamTo(fakeDestination)).To(Equal(disaster))
													})
												})
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
													reader, err := artifactSource1.StreamFile("some-path")
													Expect(err).NotTo(HaveOccurred())

													Expect(ioutil.ReadAll(reader)).To(Equal([]byte(fileContent)))

													spec := fakeContainer.StreamOutArgsForCall(0)
													Expect(spec.Path).To(Equal("/tmp/build/a1f5c0c1/some-output-configured-path/some-path"))
													Expect(spec.User).To(Equal("")) // use default
												})

												Describe("closing the stream", func() {
													It("closes the stream from the versioned source", func() {
														reader, err := artifactSource1.StreamFile("some-path")
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
													_, err := artifactSource1.StreamFile("some-path")
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
												_, err := artifactSource1.StreamFile("some-path")
												Expect(err).To(Equal(disaster))
											})
										})
									})
								})

								Context("when saving the exit status succeeds", func() {
									BeforeEach(func() {
										fakeContainer.SetPropertyReturns(nil)
									})

									It("exits successfully", func() {
										Eventually(process.Wait()).Should(Receive(BeNil()))
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

									It("does not invoke the delegate's Finished callback", func() {
										Eventually(process.Wait()).Should(Receive(Equal(disaster)))
										Expect(taskDelegate.FinishedCallCount()).To(Equal(0))
									})
								})
							})

							Context("when the process is interrupted", func() {
								var stopped chan struct{}
								BeforeEach(func() {
									stopped = make(chan struct{})

									fakeProcess.WaitStub = func() (int, error) {
										defer GinkgoRecover()

										<-stopped
										return 128 + 15, nil
									}

									fakeProcess.SignalStub = func(signal garden.Signal) error {
										switch {
										case signal == garden.SignalTerminate:
											fakeClock.IncrementBySeconds(8)
											close(stopped)
										}
										return nil
									}
								})

								It("signals the process in the container with garden.SignalTerminate", func() {
									process.Signal(os.Interrupt)
									Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))
									Expect(fakeProcess.SignalCallCount()).To(Equal(1))
									Expect(fakeProcess.SignalArgsForCall(0)).To(Equal(garden.SignalTerminate))
								})

								It("will not signal the process in the container with garden.SignalKill", func() {
									process.Signal(os.Interrupt)
									Eventually(process.Wait(), 12*time.Second).Should(Receive(Equal(ErrInterrupted)))
									Eventually(fakeProcess.SignalCallCount()).Should(Equal(1))
									Expect(fakeProcess.SignalArgsForCall(0)).To(Equal(garden.SignalTerminate))
								})

								Context("when the process doesn't exit after being signaled", func() {
									BeforeEach(func() {
										fakeProcess.SignalStub = func(signal garden.Signal) error {
											switch {
											case signal == garden.SignalTerminate:
												fakeClock.IncrementBySeconds(12)
											case signal == garden.SignalKill:
												close(stopped)
											}
											return nil
										}
									})

									It("signals the process in the container with garden.SignalKill after 10 seconds", func() {
										process.Signal(os.Interrupt)
										Eventually(process.Wait(), 12*time.Second).Should(Receive(Equal(ErrInterrupted)))
										Eventually(fakeProcess.SignalCallCount()).Should(Equal(2))
										Expect(fakeProcess.SignalArgsForCall(1)).To(Equal(garden.SignalKill))
									})
								})

								It("registers the outputs as sources", func() {
									process.Signal(os.Interrupt)
									Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))

									artifactSource1, found := repo.SourceFor("some-output")
									Expect(found).To(BeTrue())

									artifactSource2, found := repo.SourceFor("some-other-output")
									Expect(found).To(BeTrue())

									artifactSource3, found := repo.SourceFor("some-trailing-slash-output")
									Expect(found).To(BeTrue())

									sourceMap := repo.AsMap()
									Expect(sourceMap).To(ConsistOf(artifactSource1, artifactSource2, artifactSource3))
								})
							})
						})

						Context("when output is remapped", func() {
							BeforeEach(func() {
								outputMapping = map[string]string{"generic-remapped-output": "specific-remapped-output"}
								configSource.FetchConfigReturns(atc.TaskConfig{
									Run: atc.TaskRunConfig{
										Path: "ls",
									},
									Outputs: []atc.TaskOutputConfig{
										{Name: "generic-remapped-output"},
									},
								}, nil)

								fakeWorker.CreateVolumeReturns(new(wfakes.FakeVolume), nil)
								fakeProcess.WaitReturns(0, nil)
							})

							JustBeforeEach(func() {
								Eventually(process.Wait()).Should(Receive(BeNil()))
							})

							Context("when volume manager does not exist", func() {
								It("ensures the output directories exist with the generic name", func() {
									Expect(fakeContainer.StreamInCallCount()).To(Equal(2))

									spec := fakeContainer.StreamInArgsForCall(1)
									Expect(spec.Path).To(Equal("/tmp/build/a1f5c0c1/generic-remapped-output/"))
									Expect(spec.User).To(Equal("")) // use default

									tarReader := tar.NewReader(spec.TarStream)

									_, err := tarReader.Next()
									Expect(err).To(Equal(io.EOF))
								})
							})

							It("registers the outputs as sources with specific name", func() {
								artifactSource, found := repo.SourceFor("specific-remapped-output")
								Expect(found).To(BeTrue())

								sourceMap := repo.AsMap()
								Expect(sourceMap).To(ConsistOf(artifactSource))
							})

							Context("when volumes are configured", func() {
								var (
									fakeMountPath string = "/tmp/build/a1f5c0c1/generic-remapped-output/"
									fakeVolume    *wfakes.FakeVolume
								)

								BeforeEach(func() {
									fakeNewlyCreatedVolume := new(wfakes.FakeVolume)
									fakeNewlyCreatedVolume.HandleReturns("some-handle")

									fakeWorker.CreateVolumeReturns(fakeNewlyCreatedVolume, nil)

									fakeVolume = new(wfakes.FakeVolume)
									fakeVolume.HandleReturns("some-handle")

									fakeContainer.VolumeMountsReturns([]worker.VolumeMount{
										worker.VolumeMount{
											Volume:    fakeVolume,
											MountPath: fakeMountPath,
										},
									})
								})
							})
						})

						Context("when an image artifact name is specified", func() {
							BeforeEach(func() {
								imageArtifactName = "some-image-artifact"

								fakeProcess.WaitReturns(0, nil)
							})

							Context("when the image artifact is registered in the source repo", func() {
								var imageArtifactSource *fakes.FakeArtifactSource

								BeforeEach(func() {
									imageArtifactSource = new(fakes.FakeArtifactSource)
									repo.RegisterSource("some-image-artifact", imageArtifactSource)
								})

								Context("when the image artifact is not found in a volume on the worker", func() {
									var imageVolume *wfakes.FakeVolume
									var imageCowVolume *wfakes.FakeVolume
									var fakeBaggageClaimClient *bfakes.FakeClient
									dummyReader := tar.NewReader(nil)

									BeforeEach(func() {
										fakeBaggageClaimClient = new(bfakes.FakeClient)

										imageVolume = new(wfakes.FakeVolume)
										imageVolume.PathReturns("/var/vcap/some-path")
										imageVolume.HandleReturns("some-handle")

										imageCowVolume = new(wfakes.FakeVolume)
										imageCowVolume.PathReturns("/var/vcap/some-cow-path")
										imageCowVolume.HandleReturns("some-cow-handle")

										imageArtifactSource.VolumeOnReturns(nil, false, nil)

										imageVolumeChan := make(chan *wfakes.FakeVolume, 2)
										imageVolumeChan <- imageVolume
										imageVolumeChan <- imageCowVolume
										fakeWorker.CreateVolumeStub = func(logger lager.Logger, spec worker.VolumeSpec) (worker.Volume, error) {
											defer GinkgoRecover()
											if iars, ok := spec.Strategy.(worker.ContainerRootFSStrategy); ok {
												Expect(iars.Parent).To(Equal(imageVolume))
												Expect(imageVolume.ReleaseCallCount()).To(BeZero())
											}
											return <-imageVolumeChan, nil
										}
									})

									Context("when streaming the artifact source to the volume fails", func() {
										var disaster error
										BeforeEach(func() {
											disaster = errors.New("this is bad")
											imageArtifactSource.StreamToReturns(disaster)
										})

										It("returns the error", func() {
											Expect(<-process.Wait()).To(Equal(disaster))
										})

										It("releases the original volume", func() {
											Expect(imageVolume.ReleaseCallCount()).To(Equal(1))
										})
									})

									Context("when streaming the artifact source to the volume succeeds", func() {
										BeforeEach(func() {
											imageArtifactSource.StreamToReturns(nil)
										})

										Context("when streaming the metadata from the worker fails", func() {
											var disaster error
											BeforeEach(func() {
												disaster = errors.New("got em")
												imageArtifactSource.StreamFileReturns(nil, disaster)
											})

											It("returns the error", func() {
												Expect(<-process.Wait()).To(Equal(disaster))
											})

											It("releases the original volume after creating the cow volume", func() {
												// See fakeWorker.CreateVolumeStub for the rest of this assertion
												Expect(imageVolume.ReleaseCallCount()).To(Equal(1))
											})
										})

										Context("when streaming the metadata from the worker succeeds", func() {
											var metadataReader io.ReadCloser
											BeforeEach(func() {
												metadataReader = ioutil.NopCloser(strings.NewReader("some-tar-contents"))
												imageArtifactSource.StreamFileReturns(metadataReader, nil)
											})

											JustBeforeEach(func() {
												Eventually(process.Wait()).Should(Receive(BeNil()))
											})

											It("Creates a volume to stream the image into", func() {
												Expect(fakeWorker.CreateVolumeCallCount()).To(Equal(2))
												_, actualVolumeSpec := fakeWorker.CreateVolumeArgsForCall(0)
												Expect(actualVolumeSpec).To(Equal(worker.VolumeSpec{
													Strategy: worker.ImageArtifactReplicationStrategy{
														Name: imageArtifactName,
													},
													Privileged: true,
													TTL:        worker.VolumeTTL,
												}))
											})

											It("Streams the artifact source to the target volume", func() {
												Expect(imageArtifactSource.StreamToCallCount()).To(Equal(1))
												actualDest := imageArtifactSource.StreamToArgsForCall(0)
												actualDest.StreamIn(imageVolume.Handle(), dummyReader)
												Expect(imageVolume.StreamInCallCount()).To(Equal(1))
												actualHandle, actualReader := imageVolume.StreamInArgsForCall(0)
												Expect(actualHandle).To(Equal(imageVolume.Handle()))
												Expect(actualReader).To(Equal(dummyReader))
											})

											It("Creates a cow volume to set the correct privilege", func() {
												Expect(fakeWorker.CreateVolumeCallCount()).To(Equal(2))
												_, actualVolumeSpec := fakeWorker.CreateVolumeArgsForCall(1)
												Expect(actualVolumeSpec).To(Equal(worker.VolumeSpec{
													Strategy: worker.ContainerRootFSStrategy{
														Parent: imageVolume,
													},
													Privileged: false,
													TTL:        worker.VolumeTTL,
												}))
											})

											It("releases the original volume after creating the cow volume", func() {
												// See fakeWorker.CreateVolumeStub for the rest of this assertion
												Expect(imageVolume.ReleaseCallCount()).To(Equal(1))
											})

											It("streams the metadata from the worker", func() {
												Expect(imageArtifactSource.StreamFileCallCount()).To(Equal(1))
												Expect(imageArtifactSource.StreamFileArgsForCall(0)).To(Equal("metadata.json"))
											})

											It("creates the container with the volume and a metadata stream", func() {
												_, _, _, _, _, spec, _ := fakeWorker.CreateContainerArgsForCall(0)
												Expect(spec.ImageSpec).To(Equal(worker.ImageSpec{
													ImageVolumeAndMetadata: worker.ImageVolumeAndMetadata{
														Volume:         imageCowVolume,
														MetadataReader: metadataReader,
													},
												}))
											})

											Context("when the task specifies privileged", func() {
												BeforeEach(func() {
													privileged = true
												})

												It("Creates a privileged volume to stream the image into", func() {
													Expect(fakeWorker.CreateVolumeCallCount()).To(Equal(2))
													_, actualVolumeSpec := fakeWorker.CreateVolumeArgsForCall(0)
													Expect(actualVolumeSpec).To(Equal(worker.VolumeSpec{
														Strategy: worker.ImageArtifactReplicationStrategy{
															Name: imageArtifactName,
														},
														Privileged: true,
														TTL:        worker.VolumeTTL,
													}))
												})

												It("Creates a cow volume with the correct privilege", func() {
													Expect(fakeWorker.CreateVolumeCallCount()).To(Equal(2))
													_, actualVolumeSpec := fakeWorker.CreateVolumeArgsForCall(1)
													Expect(actualVolumeSpec).To(Equal(worker.VolumeSpec{
														Strategy: worker.ContainerRootFSStrategy{
															Parent: imageVolume,
														},
														Privileged: true,
														TTL:        worker.VolumeTTL,
													}))
												})
											})
										})
									})
								})

								Context("when the image artifact is in a volume on the worker", func() {
									var imageVolume *wfakes.FakeVolume
									var cowVolume *wfakes.FakeVolume

									BeforeEach(func() {
										imageVolume = new(wfakes.FakeVolume)
										imageVolume.PathReturns("/var/vcap/some-path")
										cowVolume = new(wfakes.FakeVolume)
										cowVolume.PathReturns("/var/vcap/some-cow-path")
										cowVolume.HandleReturns("cow-handle")

										fakeWorker.CreateVolumeStub = func(logger lager.Logger, spec worker.VolumeSpec) (worker.Volume, error) {
											defer GinkgoRecover()
											Expect(imageVolume.ReleaseCallCount()).To(BeZero())
											return cowVolume, nil
										}
										imageArtifactSource.VolumeOnReturns(imageVolume, true, nil)
									})

									Context("when streaming the metadata from the worker succeeds", func() {
										var metadataReader io.ReadCloser
										BeforeEach(func() {
											metadataReader = ioutil.NopCloser(strings.NewReader("some-tar-contents"))
											imageArtifactSource.StreamFileReturns(metadataReader, nil)
										})

										JustBeforeEach(func() {
											Eventually(process.Wait()).Should(Receive(BeNil()))
										})

										It("checks whether the artifact is in a volume on the worker", func() {
											Expect(imageArtifactSource.VolumeOnCallCount()).To(Equal(1))
											Expect(imageArtifactSource.VolumeOnArgsForCall(0)).To(Equal(fakeWorker))
										})

										It("creates a COW volume from the output volume", func() {
											Expect(fakeWorker.CreateVolumeCallCount()).To(Equal(1))
											_, actualVolumeSpec := fakeWorker.CreateVolumeArgsForCall(0)
											Expect(actualVolumeSpec).To(Equal(worker.VolumeSpec{
												Strategy: worker.ContainerRootFSStrategy{
													Parent: imageVolume,
												},
												Privileged: false,
												TTL:        worker.VolumeTTL,
											}))
										})

										It("releases the original volume after creating the cow volume", func() {
											// See fakeWorker.CreateVolumeStub for the rest of this assertion
											Expect(imageVolume.ReleaseCallCount()).To(Equal(1))
										})

										It("streams the metadata from the worker", func() {
											Expect(imageArtifactSource.StreamFileCallCount()).To(Equal(1))
											Expect(imageArtifactSource.StreamFileArgsForCall(0)).To(Equal("metadata.json"))
										})

										It("creates the container with the volume and a metadata stream", func() {
											_, _, _, _, _, spec, _ := fakeWorker.CreateContainerArgsForCall(0)
											Expect(spec.ImageSpec).To(Equal(worker.ImageSpec{
												ImageVolumeAndMetadata: worker.ImageVolumeAndMetadata{
													Volume:         cowVolume,
													MetadataReader: metadataReader,
												},
											}))
										})

										Context("when the task specifies privileged", func() {
											BeforeEach(func() {
												privileged = true
											})

											It("creates a privileged COW volume from the output volume", func() {
												Expect(fakeWorker.CreateVolumeCallCount()).To(Equal(1))
												_, actualVolumeSpec := fakeWorker.CreateVolumeArgsForCall(0)
												Expect(actualVolumeSpec).To(Equal(worker.VolumeSpec{
													Strategy: worker.ContainerRootFSStrategy{
														Parent: imageVolume,
													},
													Privileged: true,
													TTL:        worker.VolumeTTL,
												}))
											})
										})
									})
								})

								Describe("when task config specifies image and/or image resource as well as image artifact", func() {
									var imageVolume *wfakes.FakeVolume
									var cowVolume *wfakes.FakeVolume

									BeforeEach(func() {
										imageVolume = new(wfakes.FakeVolume)
										imageVolume.PathReturns("/var/vcap/some-path")
										imageArtifactSource.VolumeOnReturns(imageVolume, true, nil)

										cowVolume = new(wfakes.FakeVolume)
										cowVolume.PathReturns("/var/vcap/some-cow-path")
										cowVolume.HandleReturns("cow-handle")

										fakeWorker.CreateVolumeReturns(cowVolume, nil)
									})

									Context("when streaming the metadata from the worker succeeds", func() {
										var metadataReader io.ReadCloser
										BeforeEach(func() {
											metadataReader = ioutil.NopCloser(strings.NewReader("some-tar-contents"))
											imageArtifactSource.StreamFileReturns(metadataReader, nil)
										})

										JustBeforeEach(func() {
											Eventually(process.Wait()).Should(Receive(BeNil()))
										})

										Context("when the task config also specifies image", func() {
											BeforeEach(func() {
												configWithImage := atc.TaskConfig{
													Platform: "some-platform",
													Image:    "some-image",
													Params:   map[string]string{"SOME": "params"},
													Run: atc.TaskRunConfig{
														Path: "ls",
														Args: []string{"some", "args"},
													},
												}

												configSource.FetchConfigReturns(configWithImage, nil)
											})

											It("still creates the container with the volume's path as ImageURL", func() {
												_, _, _, _, _, spec, _ := fakeWorker.CreateContainerArgsForCall(0)
												Expect(spec.ImageSpec).To(Equal(worker.ImageSpec{
													ImageVolumeAndMetadata: worker.ImageVolumeAndMetadata{
														Volume:         cowVolume,
														MetadataReader: metadataReader,
													},
												}))
											})
										})

										Context("when the task config also specifies image_resource", func() {
											BeforeEach(func() {
												configWithImageResource := atc.TaskConfig{
													Platform: "some-platform",
													ImageResource: &atc.ImageResource{
														Type:   "docker",
														Source: atc.Source{"some": "source"},
													},
													Params: map[string]string{"SOME": "params"},
													Run: atc.TaskRunConfig{
														Path: "ls",
														Args: []string{"some", "args"},
													},
												}

												configSource.FetchConfigReturns(configWithImageResource, nil)
											})

											It("still creates the container with the volume's path as ImageURL", func() {
												_, _, _, _, _, spec, _ := fakeWorker.CreateContainerArgsForCall(0)
												Expect(spec.ImageSpec).To(Equal(worker.ImageSpec{
													ImageVolumeAndMetadata: worker.ImageVolumeAndMetadata{
														Volume:         cowVolume,
														MetadataReader: metadataReader,
													},
												}))
											})
										})

										Context("when the task config also specifies image and image_resource", func() {
											BeforeEach(func() {
												configWithImageAndImageResource := atc.TaskConfig{
													Platform: "some-platform",
													Image:    "some-image",
													ImageResource: &atc.ImageResource{
														Type:   "docker",
														Source: atc.Source{"some": "source"},
													},
													Params: map[string]string{"SOME": "params"},
													Run: atc.TaskRunConfig{
														Path: "ls",
														Args: []string{"some", "args"},
													},
												}

												configSource.FetchConfigReturns(configWithImageAndImageResource, nil)
											})

											It("still creates the container with the volume's path as ImageURL", func() {
												_, _, _, _, _, spec, _ := fakeWorker.CreateContainerArgsForCall(0)
												Expect(spec.ImageSpec).To(Equal(worker.ImageSpec{
													ImageVolumeAndMetadata: worker.ImageVolumeAndMetadata{
														Volume:         cowVolume,
														MetadataReader: metadataReader,
													},
												}))
											})
										})
									})
								})
							})
						})

						Context("when a run dir is specified", func() {
							BeforeEach(func() {
								fetchedConfig.Run.Dir = "/some/dir"
								configSource.FetchConfigReturns(fetchedConfig, nil)
							})

							It("runs a process in the specified (custom) directory", func() {
								spec, _ := fakeContainer.RunArgsForCall(0)
								Expect(spec.Dir).To(Equal("/tmp/build/a1f5c0c1/some/dir"))
							})
						})

						Context("when the process exits 0", func() {
							BeforeEach(func() {
								fakeProcess.WaitReturns(0, nil)
							})

							It("saves the exit status property", func() {
								<-process.Wait()

								Expect(fakeContainer.SetPropertyCallCount()).To(Equal(2))

								name, value := fakeContainer.SetPropertyArgsForCall(1)
								Expect(name).To(Equal("concourse:exit-status"))
								Expect(value).To(Equal("0"))
							})

							It("is successful", func() {
								Expect(<-process.Wait()).To(BeNil())

								var success Success
								Expect(step.Result(&success)).To(BeTrue())
								Expect(bool(success)).To(BeTrue())
							})

							It("reports its exit status", func() {
								<-process.Wait()

								var status ExitStatus
								Expect(step.Result(&status)).To(BeTrue())
								Expect(status).To(Equal(ExitStatus(0)))
							})

							Describe("release", func() {
								It("releases with the configured container success TTL", func() {
									<-process.Wait()

									step.Release()
									Expect(fakeContainer.ReleaseCallCount()).To(Equal(1))
									Expect(fakeContainer.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(worker.FinishedContainerTTL)))
								})
							})

							It("doesn't register a source", func() {
								Eventually(process.Wait()).Should(Receive(BeNil()))

								sourceMap := repo.AsMap()
								Expect(sourceMap).To(BeEmpty())
							})

							Context("when saving the exit status succeeds", func() {
								BeforeEach(func() {
									fakeContainer.SetPropertyReturns(nil)
								})

								It("exits successfully", func() {
									Eventually(process.Wait()).Should(Receive(BeNil()))
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

								It("does not invokes the delegate's Finished callback", func() {
									Eventually(process.Wait()).Should(Receive(Equal(disaster)))
									Expect(taskDelegate.FinishedCallCount()).To(Equal(0))
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

							Describe("release", func() {
								It("releases with the configured container failure TTL", func() {
									Eventually(process.Wait()).Should(Receive(BeNil()))

									step.Release()
									Expect(fakeContainer.ReleaseCallCount()).To(Equal(1))
									Expect(fakeContainer.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(worker.FinishedContainerTTL)))
								})
							})

							Context("when saving the exit status succeeds", func() {
								BeforeEach(func() {
									fakeContainer.SetPropertyReturns(nil)
								})

								It("exits successfully", func() {
									Eventually(process.Wait()).Should(Receive(BeNil()))
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

								It("does not invoke the delegate's Finished callback", func() {
									Eventually(process.Wait()).Should(Receive(Equal(disaster)))

									Expect(taskDelegate.FinishedCallCount()).To(Equal(0))
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

						Context("when the process is interrupted", func() {
							var stopped chan struct{}
							BeforeEach(func() {
								stopped = make(chan struct{})

								fakeProcess.WaitStub = func() (int, error) {
									defer GinkgoRecover()

									<-stopped
									return 128 + 15, nil
								}

								fakeProcess.SignalStub = func(signal garden.Signal) error {
									switch {
									case signal == garden.SignalTerminate:
										fakeClock.IncrementBySeconds(8)
										close(stopped)
									}
									return nil
								}
							})

							It("signals the process in the container with garden.SignalTerminate", func() {
								process.Signal(os.Interrupt)
								Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))
								Expect(fakeProcess.SignalCallCount()).To(Equal(1))
								Expect(fakeProcess.SignalArgsForCall(0)).To(Equal(garden.SignalTerminate))
							})

							It("will not signal the process in the container with garden.SignalKill", func() {
								process.Signal(os.Interrupt)
								Eventually(process.Wait(), 12*time.Second).Should(Receive(Equal(ErrInterrupted)))
								Eventually(fakeProcess.SignalCallCount()).Should(Equal(1))
								Expect(fakeProcess.SignalArgsForCall(0)).To(Equal(garden.SignalTerminate))
							})

							Context("when the process doesn't exit after being signaled", func() {
								BeforeEach(func() {
									fakeProcess.SignalStub = func(signal garden.Signal) error {
										switch {
										case signal == garden.SignalTerminate:
											fakeClock.IncrementBySeconds(12)
										case signal == garden.SignalKill:
											close(stopped)
										}
										return nil
									}
								})

								It("signals the process in the container with garden.SignalKill after 10 seconds", func() {
									process.Signal(os.Interrupt)
									Eventually(process.Wait(), 12*time.Second).Should(Receive(Equal(ErrInterrupted)))
									Eventually(fakeProcess.SignalCallCount()).Should(Equal(2))
									Expect(fakeProcess.SignalArgsForCall(1)).To(Equal(garden.SignalKill))
								})
							})

							It("doesn't register a source", func() {
								process.Signal(os.Interrupt)
								Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))

								sourceMap := repo.AsMap()
								Expect(sourceMap).To(BeEmpty())
							})
						})

						Describe("releasing", func() {
							It("releases the container", func() {
								<-process.Wait()

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

				Context("when more than one worker can be located", func() {
					var fakeWorker *wfakes.FakeWorker
					var fakeWorker2 *wfakes.FakeWorker
					var fakeWorker3 *wfakes.FakeWorker

					BeforeEach(func() {
						fakeWorker = new(wfakes.FakeWorker)
						fakeWorker2 = new(wfakes.FakeWorker)
						fakeWorker3 = new(wfakes.FakeWorker)

						fakeWorkerClient.AllSatisfyingReturns([]worker.Worker{fakeWorker, fakeWorker2, fakeWorker3}, nil)
					})

					Context("when the configuration has inputs", func() {
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
									{Name: "some-input"},
									{Name: "some-other-input"},
								},
							}, nil)
						})

						Context("when all inputs are present in the in source repository", func() {
							BeforeEach(func() {
								repo.RegisterSource("some-input", inputSource)
								repo.RegisterSource("some-other-input", otherInputSource)
							})

							Context("and some workers have more matching input volumes than others", func() {
								var rootVolume *wfakes.FakeVolume
								var inputVolume *wfakes.FakeVolume
								var inputVolume2 *wfakes.FakeVolume
								var inputVolume3 *wfakes.FakeVolume
								var otherInputVolume *wfakes.FakeVolume

								BeforeEach(func() {
									rootVolume = new(wfakes.FakeVolume)
									rootVolume.HandleReturns("root-volume")

									inputVolume = new(wfakes.FakeVolume)
									inputVolume.HandleReturns("input-volume")

									inputVolume2 = new(wfakes.FakeVolume)
									inputVolume2.HandleReturns("input-volume")

									inputVolume3 = new(wfakes.FakeVolume)
									inputVolume3.HandleReturns("input-volume")

									otherInputVolume = new(wfakes.FakeVolume)
									otherInputVolume.HandleReturns("other-input-volume")

									fakeWorker2.CreateVolumeReturns(rootVolume, nil)

									inputSource.VolumeOnStub = func(w worker.Worker) (worker.Volume, bool, error) {
										if w == fakeWorker {
											return inputVolume, true, nil
										} else if w == fakeWorker2 {
											return inputVolume2, true, nil
										} else if w == fakeWorker3 {
											return inputVolume3, true, nil
										} else {
											return nil, false, fmt.Errorf("unexpected worker: %#v\n", w)
										}
									}

									otherInputSource.VolumeOnStub = func(w worker.Worker) (worker.Volume, bool, error) {
										if w == fakeWorker {
											return nil, false, nil
										} else if w == fakeWorker2 {
											return otherInputVolume, true, nil
										} else if w == fakeWorker3 {
											return nil, false, nil
										} else {
											return nil, false, fmt.Errorf("unexpected worker: %#v\n", w)
										}
									}

									fakeWorker.CreateContainerReturns(nil, errors.New("fall out of method here"))
									fakeWorker2.CreateContainerReturns(nil, errors.New("fall out of method here"))
								})

								It("picks the worker that has the most", func() {
									Expect(fakeWorker.CreateContainerCallCount()).To(Equal(0))
									Expect(fakeWorker2.CreateContainerCallCount()).To(Equal(1))
									Expect(fakeWorker3.CreateContainerCallCount()).To(Equal(0))
								})

								It("releases the volumes on the unused workers", func() {
									Expect(inputVolume.ReleaseCallCount()).To(Equal(1))
									Expect(inputVolume3.ReleaseCallCount()).To(Equal(1))

									Expect(inputVolume2.ReleaseCallCount()).To(Equal(1))
									Expect(otherInputVolume.ReleaseCallCount()).To(Equal(1))
								})
							})
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
			Context("when the configuration specifies paths for outputs", func() {
				BeforeEach(func() {
					configSource.FetchConfigReturns(atc.TaskConfig{
						Platform: "some-platform",
						Image:    "some-image",
						Params:   map[string]string{"SOME": "params"},
						Run: atc.TaskRunConfig{
							Path: "ls",
							Args: []string{"some", "args"},
						},
						Outputs: []atc.TaskOutputConfig{
							{Name: "some-output", Path: "some-output-configured-path"},
							{Name: "some-other-output"},
							{Name: "some-trailing-slash-output", Path: "some-output-configured-path-with-trailing-slash/"},
						},
					}, nil)
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

					It("re-registers the outputs as sources", func() {
						artifactSource1, found := repo.SourceFor("some-output")
						Expect(found).To(BeTrue())

						artifactSource2, found := repo.SourceFor("some-other-output")
						Expect(found).To(BeTrue())

						artifactSource3, found := repo.SourceFor("some-trailing-slash-output")
						Expect(found).To(BeTrue())

						sourceMap := repo.AsMap()
						Expect(sourceMap).To(ConsistOf(artifactSource1, artifactSource2, artifactSource3))
					})
				})

				Context("when the process id can be found", func() {
					BeforeEach(func() {
						fakeContainer.PropertyStub = func(name string) (string, error) {
							defer GinkgoRecover()

							switch name {
							case "concourse:task-process":
								return "process-id", nil
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
							Expect(pid).To(Equal("process-id"))
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
})
