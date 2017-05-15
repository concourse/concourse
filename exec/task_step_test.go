package exec_test

import (
	"archive/tar"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/execfakes"
	"github.com/concourse/atc/resource/resourcefakes"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("GardenFactory", func() {
	var (
		fakeWorkerClient           *workerfakes.FakeClient
		fakeDBResourceCacheFactory *dbngfakes.FakeResourceCacheFactory

		factory Factory

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer
		fakeClock *fakeclock.FakeClock

		sourceName        worker.ArtifactName = "some-source-name"
		imageArtifactName string
		workerMetadata    dbng.ContainerMetadata
	)

	BeforeEach(func() {
		fakeWorkerClient = new(workerfakes.FakeClient)
		fakeResourceFactory := new(resourcefakes.FakeResourceFactory)
		fakeResourceFetcher := new(resourcefakes.FakeFetcher)
		fakeDBResourceCacheFactory = new(dbngfakes.FakeResourceCacheFactory)
		factory = NewGardenFactory(fakeWorkerClient, fakeResourceFetcher, fakeResourceFactory, fakeDBResourceCacheFactory)

		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
	})

	Describe("Task", func() {
		var (
			taskDelegate  *execfakes.FakeTaskDelegate
			privileged    Privileged
			tags          []string
			teamID        int
			configSource  *execfakes.FakeTaskConfigSource
			resourceTypes atc.VersionedResourceTypes
			inputMapping  map[string]string
			outputMapping map[string]string

			inStep *execfakes.FakeStep
			repo   *worker.ArtifactRepository

			step    Step
			process ifrit.Process
		)

		BeforeEach(func() {
			taskDelegate = new(execfakes.FakeTaskDelegate)
			taskDelegate.StdoutReturns(stdoutBuf)
			taskDelegate.StderrReturns(stderrBuf)

			privileged = false
			tags = []string{"step", "tags"}
			teamID = 123
			configSource = new(execfakes.FakeTaskConfigSource)

			inStep = new(execfakes.FakeStep)
			repo = worker.NewArtifactRepository()

			resourceTypes = atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Name:   "custom-resource",
						Type:   "custom-type",
						Source: atc.Source{"some-custom": "source"},
					},
					Version: atc.Version{"some-custom": "version"},
				},
			}

			inputMapping = nil
			outputMapping = nil
			imageArtifactName = ""
			fakeClock = fakeclock.NewFakeClock(time.Unix(0, 123))

			workerMetadata = dbng.ContainerMetadata{
				Type:     dbng.ContainerTypeTask,
				StepName: "some-step",
			}
		})

		JustBeforeEach(func() {
			step = factory.Task(
				lagertest.NewTestLogger("test"),
				teamID,
				1234,
				atc.PlanID("some-plan-id"),
				sourceName,
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

		Context("when getting the config works", func() {
			var fetchedConfig atc.TaskConfig

			BeforeEach(func() {
				fetchedConfig = atc.TaskConfig{
					Platform:  "some-platform",
					RootFsUri: "some-image",
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

			Context("when the task's container is either found or created", func() {
				var (
					fakeContainer *workerfakes.FakeContainer
				)

				BeforeEach(func() {
					fakeContainer = new(workerfakes.FakeContainer)
					fakeContainer.HandleReturns("some-handle")
					fakeWorkerClient.FindOrCreateBuildContainerReturns(fakeContainer, nil)
				})

				It("gets the config from the input artifact source", func() {
					Expect(configSource.FetchConfigCallCount()).To(Equal(1))
					Expect(configSource.FetchConfigArgsForCall(0)).To(Equal(repo))
				})

				It("finds or creates a container", func() {
					Expect(fakeWorkerClient.FindOrCreateBuildContainerCallCount()).To(Equal(1))
					_, cancel, delegate, buildID, planID, createdMetadata, spec, actualResourceTypes := fakeWorkerClient.FindOrCreateBuildContainerArgsForCall(0)
					Expect(cancel).ToNot(BeNil())
					Expect(buildID).To(Equal(1234))
					Expect(planID).To(Equal(atc.PlanID("some-plan-id")))
					Expect(createdMetadata).To(Equal(dbng.ContainerMetadata{
						Type:             dbng.ContainerTypeTask,
						StepName:         "some-step",
						WorkingDirectory: "/tmp/build/a1f5c0c1",
					}))

					Expect(delegate).To(Equal(taskDelegate))

					Expect(spec).To(Equal(worker.ContainerSpec{
						Platform: "some-platform",
						Tags:     []string{"step", "tags"},
						TeamID:   123,
						ImageSpec: worker.ImageSpec{
							ImageURL: "some-image",
							ImageResource: &atc.ImageResource{
								Type:   "docker",
								Source: atc.Source{"some": "source"},
							},
							Privileged: false,
						},
						Dir:     "/tmp/build/a1f5c0c1",
						Env:     []string{"SOME=params"},
						Inputs:  []worker.InputSource{},
						Outputs: worker.OutputPaths{},
					}))

					Expect(actualResourceTypes).To(Equal(resourceTypes))
				})

				Describe("before having created the container", func() {
					BeforeEach(func() {
						taskDelegate.InitializingStub = func(atc.TaskConfig) {
							defer GinkgoRecover()
							Expect(fakeWorkerClient.FindOrCreateBuildContainerCallCount()).To(BeZero())
						}
					})

					It("invoked the delegate's Initializing callback", func() {
						Expect(taskDelegate.InitializingCallCount()).To(Equal(1))
						Expect(taskDelegate.InitializingArgsForCall(0)).To(Equal(fetchedConfig))
					})
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

					It("is not successful as the exit status is nonzero", func() {
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

					Context("when outputs are configured and present on the container", func() {
						var (
							fakeMountPath1 string = "/tmp/build/a1f5c0c1/some-output-configured-path/"
							fakeMountPath2 string = "/tmp/build/a1f5c0c1/some-other-output/"
							fakeMountPath3 string = "/tmp/build/a1f5c0c1/some-output-configured-path-with-trailing-slash/"

							fakeNewlyCreatedVolume1 *workerfakes.FakeVolume
							fakeNewlyCreatedVolume2 *workerfakes.FakeVolume
							fakeNewlyCreatedVolume3 *workerfakes.FakeVolume

							fakeVolume1 *workerfakes.FakeVolume
							fakeVolume2 *workerfakes.FakeVolume
							fakeVolume3 *workerfakes.FakeVolume
						)

						BeforeEach(func() {
							configSource.FetchConfigReturns(atc.TaskConfig{
								Platform:  "some-platform",
								RootFsUri: "some-image",
								Params:    map[string]string{"SOME": "params"},
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

							fakeNewlyCreatedVolume1 = new(workerfakes.FakeVolume)
							fakeNewlyCreatedVolume1.HandleReturns("some-handle-1")
							fakeNewlyCreatedVolume2 = new(workerfakes.FakeVolume)
							fakeNewlyCreatedVolume2.HandleReturns("some-handle-2")
							fakeNewlyCreatedVolume3 = new(workerfakes.FakeVolume)
							fakeNewlyCreatedVolume3.HandleReturns("some-handle-3")

							fakeVolume1 = new(workerfakes.FakeVolume)
							fakeVolume1.HandleReturns("some-handle-1")
							fakeVolume2 = new(workerfakes.FakeVolume)
							fakeVolume2.HandleReturns("some-handle-2")
							fakeVolume3 = new(workerfakes.FakeVolume)
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
				})

				Context("when a process is still running", func() {
					var fakeProcess *gardenfakes.FakeProcess

					BeforeEach(func() {
						fakeContainer.PropertyReturns("", errors.New("no exit status property"))

						fakeProcess = new(gardenfakes.FakeProcess)
						fakeContainer.AttachReturns(fakeProcess, nil)
					})

					Context("when the container has task process name as its property", func() {
						BeforeEach(func() {
							fakeContainer.PropertyStub = func(propertyName string) (string, error) {
								if propertyName == "concourse:exit-status" {
									return "", errors.New("no exit status property")
								}
								if propertyName == "concourse:task-process" {
									return "some-saved-task-process", nil
								}

								panic("unknown property")
							}
						})

						It("attaches to saved process name", func() {
							Expect(fakeContainer.AttachCallCount()).To(Equal(1))

							pid, _ := fakeContainer.AttachArgsForCall(0)
							Expect(pid).To(Equal("some-saved-task-process"))
						})
					})

					Context("when the container does not have task process name as its property", func() {
						BeforeEach(func() {
							fakeContainer.PropertyStub = func(propertyName string) (string, error) {
								if propertyName == "concourse:exit-status" {
									return "", errors.New("no exit status property")
								}
								if propertyName == "concourse:task-process" {
									return "", errors.New("property does not exist")
								}

								panic("unknown property")
							}
						})

						It("attaches to saved process name", func() {
							Expect(fakeContainer.AttachCallCount()).To(Equal(1))

							pid, _ := fakeContainer.AttachArgsForCall(0)
							Expect(pid).To(Equal("task"))
						})
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

				Context("when the process is not already running or exited", func() {
					var fakeProcess *gardenfakes.FakeProcess

					BeforeEach(func() {
						fakeContainer.PropertyReturns("", errors.New("no exit status property"))
						fakeContainer.AttachReturns(nil, errors.New("no garden error type for this :("))

						fakeProcess = new(gardenfakes.FakeProcess)
						fakeContainer.RunReturns(fakeProcess, nil)
					})

					It("runs a process with the config's path and args, in the specified (default) build directory", func() {
						Expect(fakeContainer.RunCallCount()).To(Equal(1))

						spec, _ := fakeContainer.RunArgsForCall(0)
						Expect(spec.ID).To(Equal("task"))
						Expect(spec.Path).To(Equal("ls"))
						Expect(spec.Args).To(Equal([]string{"some", "args"}))
						Expect(spec.Dir).To(Equal("/tmp/build/a1f5c0c1"))
						Expect(spec.User).To(BeEmpty())
						Expect(spec.TTY).To(Equal(&garden.TTYSpec{}))
					})

					It("directs the process's stdout/stderr to the io config", func() {
						Expect(fakeContainer.RunCallCount()).To(Equal(1))

						_, io := fakeContainer.RunArgsForCall(0)
						Expect(io.Stdout).To(Equal(stdoutBuf))
						Expect(io.Stderr).To(Equal(stderrBuf))
					})

					It("invokes the delegate's Started callback", func() {
						Expect(taskDelegate.StartedCallCount()).To(Equal(1))
					})

					Context("when privileged", func() {
						BeforeEach(func() {
							privileged = true
						})

						It("creates the container privileged", func() {
							Expect(fakeWorkerClient.FindOrCreateBuildContainerCallCount()).To(Equal(1))
							_, _, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateBuildContainerArgsForCall(0)
							Expect(spec.ImageSpec.Privileged).To(BeTrue())
						})

						It("runs the process as the specified user", func() {
							Expect(fakeContainer.RunCallCount()).To(Equal(1))

							spec, _ := fakeContainer.RunArgsForCall(0)
							Expect(spec).To(Equal(garden.ProcessSpec{
								ID:   "task",
								Path: "ls",
								Args: []string{"some", "args"},
								Dir:  "/tmp/build/a1f5c0c1",
								TTY:  &garden.TTYSpec{},
							}))
						})
					})

					Context("when the configuration specifies paths for inputs", func() {
						var inputSource *workerfakes.FakeArtifactSource
						var otherInputSource *workerfakes.FakeArtifactSource

						BeforeEach(func() {
							inputSource = new(workerfakes.FakeArtifactSource)
							otherInputSource = new(workerfakes.FakeArtifactSource)

							configSource.FetchConfigReturns(atc.TaskConfig{
								Platform:  "some-platform",
								RootFsUri: "some-image",
								Params:    map[string]string{"SOME": "params"},
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

						Context("when all inputs are present", func() {
							BeforeEach(func() {
								repo.RegisterSource("some-input", inputSource)
								repo.RegisterSource("some-other-input", otherInputSource)
							})

							It("creates the container with the inputs configured correctly", func() {
								_, _, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateBuildContainerArgsForCall(0)
								Expect(spec.Inputs).To(HaveLen(2))
								for _, input := range spec.Inputs {
									switch input.Name() {
									case "some-input":
										Expect(input.DestinationPath()).To(Equal("/tmp/build/a1f5c0c1/some-input-configured-path"))
										Expect(input.Source()).To(Equal(inputSource))
									case "some-other-input":
										Expect(input.DestinationPath()).To(Equal("/tmp/build/a1f5c0c1/some-other-input"))
										Expect(input.Source()).To(Equal(otherInputSource))
									default:
										panic("unknown input: " + input.Name())
									}
								}
							})
						})

						Context("when any of the inputs are missing", func() {
							BeforeEach(func() {
								repo.RegisterSource("some-input", inputSource)
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
						var remappedInputSource *workerfakes.FakeArtifactSource

						BeforeEach(func() {
							remappedInputSource = new(workerfakes.FakeArtifactSource)
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
								_, _, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateBuildContainerArgsForCall(0)
								Expect(spec.Inputs).To(HaveLen(1))
								Expect(spec.Inputs[0].Name()).To(Equal(worker.ArtifactName("remapped-input-src")))
								Expect(spec.Inputs[0].Source()).To(Equal(remappedInputSource))
								Expect(spec.Inputs[0].DestinationPath()).To(Equal("/tmp/build/a1f5c0c1/remapped-input"))
								Eventually(process.Wait()).Should(Receive(BeNil()))
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
								Platform:  "some-platform",
								RootFsUri: "some-image",
								Params:    map[string]string{"SOME": "params"},
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

						It("configures them appropriately in the container spec", func() {
							_, _, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateBuildContainerArgsForCall(0)
							Expect(spec.Outputs).To(Equal(worker.OutputPaths{
								"some-output":                "/tmp/build/a1f5c0c1/some-output-configured-path/",
								"some-other-output":          "/tmp/build/a1f5c0c1/some-other-output/",
								"some-trailing-slash-output": "/tmp/build/a1f5c0c1/some-output-configured-path-with-trailing-slash/",
							}))
						})

						Context("when the process exits 0", func() {
							BeforeEach(func() {
								fakeProcess.WaitReturns(0, nil)
							})

							Describe("the registered sources", func() {
								var (
									artifactSource1 worker.ArtifactSource
									artifactSource2 worker.ArtifactSource
									artifactSource3 worker.ArtifactSource

									fakeMountPath1 string = "/tmp/build/a1f5c0c1/some-output-configured-path/"
									fakeMountPath2 string = "/tmp/build/a1f5c0c1/some-other-output/"
									fakeMountPath3 string = "/tmp/build/a1f5c0c1/some-output-configured-path-with-trailing-slash/"

									fakeNewlyCreatedVolume1 *workerfakes.FakeVolume
									fakeNewlyCreatedVolume2 *workerfakes.FakeVolume
									fakeNewlyCreatedVolume3 *workerfakes.FakeVolume

									fakeVolume1 *workerfakes.FakeVolume
									fakeVolume2 *workerfakes.FakeVolume
									fakeVolume3 *workerfakes.FakeVolume
								)

								BeforeEach(func() {
									fakeNewlyCreatedVolume1 = new(workerfakes.FakeVolume)
									fakeNewlyCreatedVolume1.HandleReturns("some-handle-1")
									fakeNewlyCreatedVolume2 = new(workerfakes.FakeVolume)
									fakeNewlyCreatedVolume2.HandleReturns("some-handle-2")
									fakeNewlyCreatedVolume3 = new(workerfakes.FakeVolume)
									fakeNewlyCreatedVolume3.HandleReturns("some-handle-3")

									fakeVolume1 = new(workerfakes.FakeVolume)
									fakeVolume1.HandleReturns("some-handle-1")
									fakeVolume2 = new(workerfakes.FakeVolume)
									fakeVolume2.HandleReturns("some-handle-2")
									fakeVolume3 = new(workerfakes.FakeVolume)
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
								})

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
									var streamedOut io.ReadCloser
									var fakeDestination *workerfakes.FakeArtifactDestination

									BeforeEach(func() {
										fakeDestination = new(workerfakes.FakeArtifactDestination)

										streamedOut = gbytes.NewBuffer()
										fakeVolume1.StreamOutReturns(streamedOut, nil)
									})

									It("passes existing output volumes to the resource", func() {
										_, _, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateBuildContainerArgsForCall(0)
										Expect(spec.Outputs).To(Equal(worker.OutputPaths{
											"some-output":                "/tmp/build/a1f5c0c1/some-output-configured-path/",
											"some-other-output":          "/tmp/build/a1f5c0c1/some-other-output/",
											"some-trailing-slash-output": "/tmp/build/a1f5c0c1/some-output-configured-path-with-trailing-slash/",
										}))
									})

									It("streams the data from the volumes to the destination", func() {
										err := artifactSource1.StreamTo(fakeDestination)
										Expect(err).NotTo(HaveOccurred())

										Expect(fakeVolume1.StreamOutCallCount()).To(Equal(1))
										path := fakeVolume1.StreamOutArgsForCall(0)
										Expect(path).To(Equal("."))

										Expect(fakeDestination.StreamInCallCount()).To(Equal(1))
										dest, src := fakeDestination.StreamInArgsForCall(0)
										Expect(dest).To(Equal("."))
										Expect(src).To(Equal(streamedOut))
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
											fakeVolume1.StreamOutReturns(tarBuffer, nil)
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

												path := fakeVolume1.StreamOutArgsForCall(0)
												Expect(path).To(Equal("some-path"))
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

									Context("when the volume cannot stream out", func() {
										disaster := errors.New("nope")

										BeforeEach(func() {
											fakeVolume1.StreamOutReturns(nil, disaster)
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

								fakeContainer.StopStub = func(bool) error {
									close(stopped)
									return nil
								}
							})

							It("stops the container", func() {
								process.Signal(os.Interrupt)
								Eventually(fakeContainer.StopCallCount, 8*time.Second).Should(Equal(1))
								Expect(fakeContainer.StopArgsForCall(0)).To(BeFalse())
								Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))
							})

							Context("when container.stop returns an error", func() {
								var disaster error

								BeforeEach(func() {
									disaster = errors.New("gotta get away")

									fakeContainer.StopStub = func(bool) error {
										close(stopped)
										return disaster
									}
								})

								It("doesn't return the error", func() {
									process.Signal(os.Interrupt)
									Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))
								})
							})

							Context("when volume mounts are present on the container", func() {
								var (
									fakeMountPath1 string = "/tmp/build/a1f5c0c1/some-output-configured-path/"
									fakeMountPath2 string = "/tmp/build/a1f5c0c1/some-other-output/"
									fakeMountPath3 string = "/tmp/build/a1f5c0c1/some-output-configured-path-with-trailing-slash/"

									fakeNewlyCreatedVolume1 *workerfakes.FakeVolume
									fakeNewlyCreatedVolume2 *workerfakes.FakeVolume
									fakeNewlyCreatedVolume3 *workerfakes.FakeVolume

									fakeVolume1 *workerfakes.FakeVolume
									fakeVolume2 *workerfakes.FakeVolume
									fakeVolume3 *workerfakes.FakeVolume
								)

								BeforeEach(func() {
									fakeNewlyCreatedVolume1 = new(workerfakes.FakeVolume)
									fakeNewlyCreatedVolume1.HandleReturns("some-handle-1")
									fakeNewlyCreatedVolume2 = new(workerfakes.FakeVolume)
									fakeNewlyCreatedVolume2.HandleReturns("some-handle-2")
									fakeNewlyCreatedVolume3 = new(workerfakes.FakeVolume)
									fakeNewlyCreatedVolume3.HandleReturns("some-handle-3")

									fakeVolume1 = new(workerfakes.FakeVolume)
									fakeVolume1.HandleReturns("some-handle-1")
									fakeVolume2 = new(workerfakes.FakeVolume)
									fakeVolume2.HandleReturns("some-handle-2")
									fakeVolume3 = new(workerfakes.FakeVolume)
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
					})

					Context("when output is remapped", func() {
						var (
							fakeMountPath string = "/tmp/build/a1f5c0c1/generic-remapped-output/"
						)

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

							fakeProcess.WaitReturns(0, nil)

							fakeVolume := new(workerfakes.FakeVolume)
							fakeVolume.HandleReturns("some-handle")

							fakeContainer.VolumeMountsReturns([]worker.VolumeMount{
								worker.VolumeMount{
									Volume:    fakeVolume,
									MountPath: fakeMountPath,
								},
							})
						})

						JustBeforeEach(func() {
							Eventually(process.Wait()).Should(Receive(BeNil()))
						})

						It("registers the outputs as sources with specific name", func() {
							artifactSource, found := repo.SourceFor("specific-remapped-output")
							Expect(found).To(BeTrue())

							sourceMap := repo.AsMap()
							Expect(sourceMap).To(ConsistOf(artifactSource))
						})
					})

					Context("when an image artifact name is specified", func() {
						BeforeEach(func() {
							imageArtifactName = "some-image-artifact"

							fakeProcess.WaitReturns(0, nil)
						})

						Context("when the image artifact is registered in the source repo", func() {
							var imageArtifactSource *workerfakes.FakeArtifactSource

							BeforeEach(func() {
								imageArtifactSource = new(workerfakes.FakeArtifactSource)
								repo.RegisterSource("some-image-artifact", imageArtifactSource)
							})

							It("creates the container with the image artifact source", func() {
								_, _, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateBuildContainerArgsForCall(0)
								Expect(spec.ImageSpec).To(Equal(worker.ImageSpec{
									ImageArtifactSource: imageArtifactSource,
									ImageArtifactName:   worker.ArtifactName(imageArtifactName),
								}))
							})

							Describe("when task config specifies image and/or image resource as well as image artifact", func() {
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
												Platform:  "some-platform",
												RootFsUri: "some-image",
												Params:    map[string]string{"SOME": "params"},
												Run: atc.TaskRunConfig{
													Path: "ls",
													Args: []string{"some", "args"},
												},
											}

											configSource.FetchConfigReturns(configWithImage, nil)
										})

										It("still creates the container with the volume and a metadata stream", func() {
											_, _, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateBuildContainerArgsForCall(0)
											Expect(spec.ImageSpec).To(Equal(worker.ImageSpec{
												ImageArtifactSource: imageArtifactSource,
												ImageArtifactName:   worker.ArtifactName(imageArtifactName),
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

										It("still creates the container with the volume and a metadata stream", func() {
											_, _, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateBuildContainerArgsForCall(0)
											Expect(spec.ImageSpec).To(Equal(worker.ImageSpec{
												ImageArtifactSource: imageArtifactSource,
												ImageArtifactName:   worker.ArtifactName(imageArtifactName),
											}))
										})
									})

									Context("when the task config also specifies image and image_resource", func() {
										BeforeEach(func() {
											configWithImageAndImageResource := atc.TaskConfig{
												Platform:  "some-platform",
												RootFsUri: "some-image",
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

										It("still creates the container with the volume and a metadata stream", func() {
											_, _, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateBuildContainerArgsForCall(0)
											Expect(spec.ImageSpec).To(Equal(worker.ImageSpec{
												ImageArtifactSource: imageArtifactSource,
												ImageArtifactName:   worker.ArtifactName(imageArtifactName),
											}))
										})
									})
								})
							})
						})

						Context("when the image artifact is NOT registered in the source repo", func() {
							It("exits with the MissingTaskImageSourceError", func() {
								Eventually(process.Wait()).Should(Receive(Equal(MissingTaskImageSourceError{"some-image-artifact"})))
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

					Context("when a run user is specified", func() {
						BeforeEach(func() {
							fetchedConfig.Run.User = "some-user"
							configSource.FetchConfigReturns(fetchedConfig, nil)
						})

						It("adds the user to the container spec", func() {
							_, _, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateBuildContainerArgsForCall(0)
							Expect(spec.User).To(Equal("some-user"))
						})

						It("doesn't bother adding the user to the run spec", func() {
							spec, _ := fakeContainer.RunArgsForCall(0)
							Expect(spec.User).To(BeEmpty())
						})
					})

					Context("when the process exits 0", func() {
						BeforeEach(func() {
							fakeProcess.WaitReturns(0, nil)
						})

						It("saves the exit status property", func() {
							<-process.Wait()

							Expect(fakeContainer.SetPropertyCallCount()).To(Equal(1))

							name, value := fakeContainer.SetPropertyArgsForCall(0)
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

							Expect(fakeContainer.SetPropertyCallCount()).To(Equal(1))

							name, value := fakeContainer.SetPropertyArgsForCall(0)
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

					Context("when the process is interrupted", func() {
						var stopped chan struct{}
						BeforeEach(func() {
							stopped = make(chan struct{})

							fakeProcess.WaitStub = func() (int, error) {
								defer GinkgoRecover()

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
							Eventually(fakeContainer.StopCallCount, 8*time.Second).Should(Equal(1))
							Expect(fakeContainer.StopArgsForCall(0)).To(BeFalse())
							Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))
						})

						Context("when container.stop returns an error", func() {
							var disaster error

							BeforeEach(func() {
								disaster = errors.New("gotta get away")

								fakeContainer.StopStub = func(bool) error {
									close(stopped)
									return disaster
								}
							})

							It("doesn't return the error", func() {
								process.Signal(os.Interrupt)
								Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))
							})
						})

						It("doesn't register a source", func() {
							process.Signal(os.Interrupt)
							Eventually(process.Wait()).Should(Receive(Equal(ErrInterrupted)))

							sourceMap := repo.AsMap()
							Expect(sourceMap).To(BeEmpty())
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
			})

			Context("when creating the container fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeWorkerClient.FindOrCreateBuildContainerReturns(nil, disaster)
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
})
