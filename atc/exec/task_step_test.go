package exec_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/execfakes"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("TaskStep", func() {
	var (
		ctx    context.Context
		cancel func()

		fakeWorkerClient *workerfakes.FakeClient

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		imageArtifactName string
		containerMetadata db.ContainerMetadata

		fakeDelegate *execfakes.FakeTaskDelegate

		privileged    exec.Privileged
		tags          []string
		teamID        int
		buildID       int
		planID        atc.PlanID
		jobID         int
		configSource  *execfakes.FakeTaskConfigSource
		resourceTypes creds.VersionedResourceTypes
		inputMapping  map[string]string
		outputMapping map[string]string
		variables     creds.Variables

		repo  *worker.ArtifactRepository
		state *execfakes.FakeRunState

		taskStep exec.Step

		stepErr error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		fakeWorkerClient = new(workerfakes.FakeClient)

		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()

		fakeDelegate = new(execfakes.FakeTaskDelegate)
		fakeDelegate.StdoutReturns(stdoutBuf)
		fakeDelegate.StderrReturns(stderrBuf)

		privileged = false
		tags = []string{"step", "tags"}
		teamID = 123
		planID = atc.PlanID(42)
		buildID = 1234
		jobID = 12345
		configSource = new(execfakes.FakeTaskConfigSource)

		repo = worker.NewArtifactRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactsReturns(repo)

		resourceTypes = creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-resource",
					Type:   "custom-type",
					Source: atc.Source{"some-custom": "source"},
					Params: atc.Params{"some-custom": "param"},
				},
				Version: atc.Version{"some-custom": "version"},
			},
		})

		inputMapping = nil
		outputMapping = nil
		imageArtifactName = ""

		variables = template.StaticVariables{
			"source-param": "super-secret-source",
			"task-param":   "super-secret-param",
		}

		containerMetadata = db.ContainerMetadata{
			Type:     db.ContainerTypeTask,
			StepName: "some-step",
		}

		stepErr = nil
	})

	JustBeforeEach(func() {
		taskStep = exec.NewTaskStep(
			privileged,
			configSource,
			tags,
			inputMapping,
			outputMapping,
			"some-artifact-root",
			imageArtifactName,
			fakeDelegate,
			fakeWorkerClient,
			teamID,
			buildID,
			jobID,
			"some-task",
			planID,
			containerMetadata,
			resourceTypes,
			variables,
			atc.ContainerLimits{},
		)

		stepErr = taskStep.Run(ctx, state)
	})

	Context("when getting the config works", func() {
		var fetchedConfig atc.TaskConfig

		BeforeEach(func() {
			cpu := uint64(1024)
			memory := uint64(1024)
			fetchedConfig = atc.TaskConfig{
				Platform: "some-platform",
				ImageResource: &atc.ImageResource{
					Type:    "docker",
					Source:  atc.Source{"some": "((source-param))"},
					Params:  &atc.Params{"some": "params"},
					Version: &atc.Version{"some": "version"},
				},
				Limits: atc.ContainerLimits{
					CPU:    &cpu,
					Memory: &memory,
				},
				Params: map[string]string{
					"SECURE": "((task-param))",
				},
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
				fakeWorkerClient.FindOrCreateContainerReturns(fakeContainer, nil)
			})

			Describe("before creating a container", func() {
				BeforeEach(func() {
					fakeDelegate.InitializingStub = func(lager.Logger, atc.TaskConfig) {
						defer GinkgoRecover()
						Expect(fakeWorkerClient.FindOrCreateContainerCallCount()).To(BeZero())
					}
				})

				It("invoked the delegate's Initializing callback", func() {
					Expect(fakeDelegate.InitializingCallCount()).To(Equal(1))
				})
			})

			It("finds or creates a container", func() {
				Expect(fakeWorkerClient.FindOrCreateContainerCallCount()).To(Equal(1))
				_, cancel, delegate, owner, createdMetadata, spec, actualResourceTypes := fakeWorkerClient.FindOrCreateContainerArgsForCall(0)
				Expect(cancel).ToNot(BeNil())
				Expect(owner).To(Equal(db.NewBuildStepContainerOwner(buildID, planID)))
				Expect(createdMetadata).To(Equal(db.ContainerMetadata{
					Type:     db.ContainerTypeTask,
					StepName: "some-step",
				}))

				Expect(delegate).To(Equal(fakeDelegate))

				cpu := uint64(1024)
				memory := uint64(1024)
				Expect(spec).To(Equal(worker.ContainerSpec{
					Platform: "some-platform",
					Tags:     []string{"step", "tags"},
					TeamID:   teamID,
					ImageSpec: worker.ImageSpec{
						ImageResource: &worker.ImageResource{
							Type:    "docker",
							Source:  creds.NewSource(variables, atc.Source{"some": "((source-param))"}),
							Params:  &atc.Params{"some": "params"},
							Version: &atc.Version{"some": "version"},
						},
						Privileged: false,
					},
					Limits: worker.ContainerLimits{
						CPU:    &cpu,
						Memory: &memory,
					},
					Dir:     "some-artifact-root",
					Env:     []string{"SECURE=super-secret-param"},
					Inputs:  []worker.InputSource{},
					Outputs: worker.OutputPaths{},
				}))
				Expect(actualResourceTypes).To(Equal(resourceTypes))
			})

			Context("when rootfs uri is set instead of image resource", func() {
				BeforeEach(func() {
					fetchedConfig = atc.TaskConfig{
						Platform:  "some-platform",
						RootfsURI: "some-image",
						Params:    map[string]string{"SOME": "params"},
						Run: atc.TaskRunConfig{
							Path: "ls",
							Args: []string{"some", "args"},
						},
					}

					configSource.FetchConfigReturns(fetchedConfig, nil)
				})

				It("finds or creates a container", func() {
					Expect(fakeWorkerClient.FindOrCreateContainerCallCount()).To(Equal(1))
					_, cancel, delegate, owner, createdMetadata, spec, actualResourceTypes := fakeWorkerClient.FindOrCreateContainerArgsForCall(0)
					Expect(cancel).ToNot(BeNil())
					Expect(owner).To(Equal(db.NewBuildStepContainerOwner(buildID, planID)))
					Expect(createdMetadata).To(Equal(db.ContainerMetadata{
						Type:     db.ContainerTypeTask,
						StepName: "some-step",
					}))

					Expect(delegate).To(Equal(fakeDelegate))

					Expect(spec).To(Equal(worker.ContainerSpec{
						Platform: "some-platform",
						Tags:     []string{"step", "tags"},
						TeamID:   teamID,
						ImageSpec: worker.ImageSpec{
							ImageURL:   "some-image",
							Privileged: false,
						},
						Dir:     "some-artifact-root",
						Env:     []string{"SOME=params"},
						Inputs:  []worker.InputSource{},
						Outputs: worker.OutputPaths{},
					}))

					Expect(actualResourceTypes).To(Equal(resourceTypes))
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

				It("returns no error", func() {
					Expect(stepErr).ToNot(HaveOccurred())
				})

				It("does not attach to any process", func() {
					Expect(fakeContainer.AttachCallCount()).To(BeZero())
				})

				It("is not successful as the exit status is nonzero", func() {
					Expect(taskStep.Succeeded()).To(BeFalse())
				})

				Context("when outputs are configured and present on the container", func() {
					var (
						fakeMountPath1 string = "some-artifact-root/some-output-configured-path/"
						fakeMountPath2 string = "some-artifact-root/some-other-output/"
						fakeMountPath3 string = "some-artifact-root/some-output-configured-path-with-trailing-slash/"

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
							RootfsURI: "some-image",
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
			})

			Context("when the process is not already running or exited", func() {
				var fakeProcess *gardenfakes.FakeProcess

				BeforeEach(func() {
					fakeContainer.PropertyReturns("", errors.New("no exit status property"))
					fakeContainer.AttachReturns(nil, errors.New("no garden error type for this :("))

					fakeProcess = new(gardenfakes.FakeProcess)
					fakeContainer.RunReturns(fakeProcess, nil)
				})

				Describe("before running a process", func() {
					BeforeEach(func() {
						fakeDelegate.StartingStub = func(lager.Logger, atc.TaskConfig) {
							defer GinkgoRecover()
							Expect(fakeContainer.RunCallCount()).To(BeZero())
						}
					})

					It("invoked the delegate's Starting callback", func() {
						Expect(fakeDelegate.StartingCallCount()).To(Equal(1))
					})
				})

				It("runs a process with the config's path and args, in the specified (default) build directory", func() {
					Expect(fakeContainer.RunCallCount()).To(Equal(1))

					spec, _ := fakeContainer.RunArgsForCall(0)
					Expect(spec.ID).To(Equal("task"))
					Expect(spec.Path).To(Equal("ls"))
					Expect(spec.Args).To(Equal([]string{"some", "args"}))
					Expect(spec.Dir).To(Equal("some-artifact-root"))
					Expect(spec.User).To(BeEmpty())
					Expect(spec.TTY).To(Equal(&garden.TTYSpec{WindowSize: &garden.WindowSize{Columns: 500, Rows: 500}}))
				})

				It("directs the process's stdout/stderr to the io config", func() {
					Expect(fakeContainer.RunCallCount()).To(Equal(1))

					_, io := fakeContainer.RunArgsForCall(0)
					Expect(io.Stdout).To(Equal(stdoutBuf))
					Expect(io.Stderr).To(Equal(stderrBuf))
				})

				Context("when privileged", func() {
					BeforeEach(func() {
						privileged = true
					})

					It("creates the container privileged", func() {
						Expect(fakeWorkerClient.FindOrCreateContainerCallCount()).To(Equal(1))
						_, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateContainerArgsForCall(0)
						Expect(spec.ImageSpec.Privileged).To(BeTrue())
					})

					It("runs the process as the specified user", func() {
						Expect(fakeContainer.RunCallCount()).To(Equal(1))

						spec, _ := fakeContainer.RunArgsForCall(0)
						Expect(spec).To(Equal(garden.ProcessSpec{
							ID:   "task",
							Path: "ls",
							Args: []string{"some", "args"},
							Dir:  "some-artifact-root",
							TTY:  &garden.TTYSpec{WindowSize: &garden.WindowSize{Columns: 500, Rows: 500}},
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
							RootfsURI: "some-image",
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
							_, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateContainerArgsForCall(0)
							Expect(spec.Inputs).To(HaveLen(2))
							for _, input := range spec.Inputs {
								switch input.DestinationPath() {
								case "some-artifact-root/some-input-configured-path":
									Expect(input.Source()).To(Equal(inputSource))
								case "some-artifact-root/some-other-input":
									Expect(input.Source()).To(Equal(otherInputSource))
								default:
									panic("unknown input: " + input.DestinationPath())
								}
							}
						})
					})

					Context("when any of the inputs are missing", func() {
						BeforeEach(func() {
							repo.RegisterSource("some-input", inputSource)
						})

						It("returns a MissingInputsError", func() {
							Expect(stepErr).To(BeAssignableToTypeOf(exec.MissingInputsError{}))
							Expect(stepErr.(exec.MissingInputsError).Inputs).To(ConsistOf("some-other-input"))
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
							_, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateContainerArgsForCall(0)
							Expect(spec.Inputs).To(HaveLen(1))
							Expect(spec.Inputs[0].Source()).To(Equal(remappedInputSource))
							Expect(spec.Inputs[0].DestinationPath()).To(Equal("some-artifact-root/remapped-input"))
							Expect(stepErr).ToNot(HaveOccurred())
						})
					})

					Context("when any of the inputs are missing", func() {
						It("returns a MissingInputsError", func() {
							Expect(stepErr).To(BeAssignableToTypeOf(exec.MissingInputsError{}))
							Expect(stepErr.(exec.MissingInputsError).Inputs).To(ConsistOf("remapped-input-src"))
						})
					})
				})

				Context("when some inputs are optional", func() {
					var (
						optionalInputSource, optionalInput2Source, requiredInputSource *workerfakes.FakeArtifactSource
					)

					BeforeEach(func() {
						optionalInputSource = new(workerfakes.FakeArtifactSource)
						optionalInput2Source = new(workerfakes.FakeArtifactSource)
						requiredInputSource = new(workerfakes.FakeArtifactSource)
						configSource.FetchConfigReturns(atc.TaskConfig{
							Run: atc.TaskRunConfig{
								Path: "ls",
							},
							Inputs: []atc.TaskInputConfig{
								{Name: "optional-input", Optional: true},
								{Name: "optional-input-2", Optional: true},
								{Name: "required-input"},
							},
						}, nil)
					})

					Context("when an optional input is missing", func() {
						BeforeEach(func() {
							repo.RegisterSource("required-input", requiredInputSource)
							repo.RegisterSource("optional-input-2", optionalInput2Source)
						})

						It("runs successfully without the optional input", func() {
							Expect(stepErr).ToNot(HaveOccurred())
							_, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateContainerArgsForCall(0)
							Expect(spec.Inputs).To(HaveLen(2))
							Expect(spec.Inputs[0].Source()).To(Equal(optionalInput2Source))
							Expect(spec.Inputs[0].DestinationPath()).To(Equal("some-artifact-root/optional-input-2"))
							Expect(spec.Inputs[1].Source()).To(Equal(requiredInputSource))
							Expect(spec.Inputs[1].DestinationPath()).To(Equal("some-artifact-root/required-input"))
						})
					})

					Context("when a required input is missing", func() {
						BeforeEach(func() {
							repo.RegisterSource("optional-input", optionalInputSource)
							repo.RegisterSource("optional-input-2", optionalInput2Source)
						})

						It("returns a MissingInputsError", func() {
							Expect(stepErr).To(BeAssignableToTypeOf(exec.MissingInputsError{}))
							Expect(stepErr.(exec.MissingInputsError).Inputs).To(ConsistOf("required-input"))
						})
					})
				})

				Context("when the configuration specifies paths for caches", func() {
					var (
						fakeVolume1 *workerfakes.FakeVolume
						fakeVolume2 *workerfakes.FakeVolume
					)

					BeforeEach(func() {
						configSource.FetchConfigReturns(atc.TaskConfig{
							Platform:  "some-platform",
							RootfsURI: "some-image",
							Run:       atc.TaskRunConfig{},
							Caches: []atc.CacheConfig{
								{Path: "some-path-1"},
								{Path: "some-path-2"},
							},
						}, nil)

						fakeVolume1 = new(workerfakes.FakeVolume)
						fakeVolume2 = new(workerfakes.FakeVolume)
						fakeContainer.VolumeMountsReturns([]worker.VolumeMount{
							worker.VolumeMount{
								Volume:    fakeVolume1,
								MountPath: "some-artifact-root/some-path-1",
							},
							worker.VolumeMount{
								Volume:    fakeVolume2,
								MountPath: "some-artifact-root/some-path-2",
							},
						})
					})

					It("creates the container with the caches in the inputs", func() {
						_, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateContainerArgsForCall(0)
						Expect(spec.Inputs).To(HaveLen(2))
						Expect([]string{
							spec.Inputs[0].DestinationPath(),
							spec.Inputs[1].DestinationPath(),
						}).To(ConsistOf(
							"some-artifact-root/some-path-1",
							"some-artifact-root/some-path-2",
						))
					})

					It("registers cache volumes as task caches", func() {
						Expect(stepErr).ToNot(HaveOccurred())

						Expect(fakeVolume1.InitializeTaskCacheCallCount()).To(Equal(1))
						_, jID, stepName, cachePath, p := fakeVolume1.InitializeTaskCacheArgsForCall(0)
						Expect(jID).To(Equal(jobID))
						Expect(stepName).To(Equal("some-task"))
						Expect(cachePath).To(Equal("some-path-1"))
						Expect(p).To(Equal(bool(privileged)))

						Expect(fakeVolume2.InitializeTaskCacheCallCount()).To(Equal(1))
						_, jID, stepName, cachePath, p = fakeVolume2.InitializeTaskCacheArgsForCall(0)
						Expect(jID).To(Equal(jobID))
						Expect(stepName).To(Equal("some-task"))
						Expect(cachePath).To(Equal("some-path-2"))
						Expect(p).To(Equal(bool(privileged)))
					})

					Context("when task does not belong to job (one-off build)", func() {
						BeforeEach(func() {
							jobID = 0
						})

						It("does not initialize caches", func() {
							Expect(stepErr).ToNot(HaveOccurred())
							Expect(fakeVolume1.InitializeTaskCacheCallCount()).To(Equal(0))
							Expect(fakeVolume2.InitializeTaskCacheCallCount()).To(Equal(0))
						})
					})
				})

				Context("when the configuration specifies paths for outputs", func() {
					BeforeEach(func() {
						configSource.FetchConfigReturns(atc.TaskConfig{
							Platform:  "some-platform",
							RootfsURI: "some-image",
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
						_, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateContainerArgsForCall(0)
						Expect(spec.Outputs).To(Equal(worker.OutputPaths{
							"some-output":                "some-artifact-root/some-output-configured-path/",
							"some-other-output":          "some-artifact-root/some-other-output/",
							"some-trailing-slash-output": "some-artifact-root/some-output-configured-path-with-trailing-slash/",
						}))
					})

					Context("when the process exits 0", func() {
						BeforeEach(func() {
							fakeProcess.WaitReturns(0, nil)
						})

						It("finishes the task via the delegate", func() {
							Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
							_, status := fakeDelegate.FinishedArgsForCall(0)
							Expect(status).To(Equal(exec.ExitStatus(0)))
						})

						Describe("the registered sources", func() {
							var (
								artifactSource1 worker.ArtifactSource
								artifactSource2 worker.ArtifactSource
								artifactSource3 worker.ArtifactSource

								fakeMountPath1 string = "some-artifact-root/some-output-configured-path/"
								fakeMountPath2 string = "some-artifact-root/some-other-output/"
								fakeMountPath3 string = "some-artifact-root/some-output-configured-path-with-trailing-slash/"

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
								Expect(stepErr).ToNot(HaveOccurred())

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
									_, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateContainerArgsForCall(0)
									Expect(spec.Outputs).To(Equal(worker.OutputPaths{
										"some-output":                "some-artifact-root/some-output-configured-path/",
										"some-other-output":          "some-artifact-root/some-other-output/",
										"some-trailing-slash-output": "some-artifact-root/some-output-configured-path-with-trailing-slash/",
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

										tgzBuffer *gbytes.Buffer
									)

									BeforeEach(func() {
										tgzBuffer = gbytes.NewBuffer()
										fakeVolume1.StreamOutReturns(tgzBuffer, nil)
									})

									Context("when the file exists", func() {
										BeforeEach(func() {
											gzWriter := gzip.NewWriter(tgzBuffer)
											defer gzWriter.Close()

											tarWriter := tar.NewWriter(gzWriter)
											defer tarWriter.Close()

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

												Expect(tgzBuffer.Closed()).To(BeFalse())

												err = reader.Close()
												Expect(err).NotTo(HaveOccurred())

												Expect(tgzBuffer.Closed()).To(BeTrue())
											})
										})
									})

									Context("but the stream is empty", func() {
										It("returns ErrFileNotFound", func() {
											_, err := artifactSource1.StreamFile("some-path")
											Expect(err).To(MatchError(exec.FileNotFoundError{Path: "some-path"}))
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

							It("returns successfully", func() {
								Expect(stepErr).ToNot(HaveOccurred())
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

							It("returns the error", func() {
								Expect(stepErr).To(Equal(disaster))
							})

							It("is not successful", func() {
								Expect(taskStep.Succeeded()).To(BeFalse())
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

							cancel()
						})

						It("stops the container", func() {
							Expect(fakeContainer.StopCallCount()).To(Equal(1))
							Expect(fakeContainer.StopArgsForCall(0)).To(BeFalse())
							Expect(stepErr).To(Equal(context.Canceled))
						})

						It("is not successful", func() {
							Expect(taskStep.Succeeded()).To(BeFalse())
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
								Expect(stepErr).To(Equal(context.Canceled))
							})

							It("is not successful", func() {
								Expect(taskStep.Succeeded()).To(BeFalse())
							})
						})

						Context("when volume mounts are present on the container", func() {
							var (
								fakeMountPath1 string = "some-artifact-root/some-output-configured-path/"
								fakeMountPath2 string = "some-artifact-root/some-other-output/"
								fakeMountPath3 string = "some-artifact-root/some-output-configured-path-with-trailing-slash/"

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
								cancel()
								Expect(stepErr).To(Equal(context.Canceled))

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
						fakeMountPath string = "some-artifact-root/generic-remapped-output/"
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
						Expect(stepErr).ToNot(HaveOccurred())
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
							_, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateContainerArgsForCall(0)
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
									Expect(stepErr).ToNot(HaveOccurred())
								})

								Context("when the task config also specifies image", func() {
									BeforeEach(func() {
										configWithImage := atc.TaskConfig{
											Platform:  "some-platform",
											RootfsURI: "some-image",
											Params:    map[string]string{"SOME": "params"},
											Run: atc.TaskRunConfig{
												Path: "ls",
												Args: []string{"some", "args"},
											},
										}

										configSource.FetchConfigReturns(configWithImage, nil)
									})

									It("still creates the container with the volume and a metadata stream", func() {
										_, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateContainerArgsForCall(0)
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
												Type:    "docker",
												Source:  atc.Source{"some": "super-secret-source"},
												Params:  &atc.Params{"some": "params"},
												Version: &atc.Version{"some": "version"},
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
										_, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateContainerArgsForCall(0)
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
											RootfsURI: "some-image",
											ImageResource: &atc.ImageResource{
												Type:    "docker",
												Source:  atc.Source{"some": "super-secret-source"},
												Params:  &atc.Params{"some": "params"},
												Version: &atc.Version{"some": "version"},
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
										_, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateContainerArgsForCall(0)
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
						It("returns a MissingTaskImageSourceError", func() {
							Expect(stepErr).To(Equal(exec.MissingTaskImageSourceError{"some-image-artifact"}))
						})

						It("is not successful", func() {
							Expect(taskStep.Succeeded()).To(BeFalse())
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
						Expect(spec.Dir).To(Equal("some-artifact-root/some/dir"))
					})
				})

				Context("when a run user is specified", func() {
					BeforeEach(func() {
						fetchedConfig.Run.User = "some-user"
						configSource.FetchConfigReturns(fetchedConfig, nil)
					})

					It("adds the user to the container spec", func() {
						_, _, _, _, _, spec, _ := fakeWorkerClient.FindOrCreateContainerArgsForCall(0)
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
						Expect(fakeContainer.SetPropertyCallCount()).To(Equal(1))

						name, value := fakeContainer.SetPropertyArgsForCall(0)
						Expect(name).To(Equal("concourse:exit-status"))
						Expect(value).To(Equal("0"))
					})

					It("is successful", func() {
						Expect(stepErr).To(BeNil())
						Expect(taskStep.Succeeded()).To(BeTrue())
					})

					It("doesn't register a source", func() {
						Expect(stepErr).ToNot(HaveOccurred())

						sourceMap := repo.AsMap()
						Expect(sourceMap).To(BeEmpty())
					})

					Context("when saving the exit status succeeds", func() {
						BeforeEach(func() {
							fakeContainer.SetPropertyReturns(nil)
						})

						It("returns successfully", func() {
							Expect(stepErr).ToNot(HaveOccurred())
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

						It("returns the error", func() {
							Expect(stepErr).To(Equal(disaster))
						})

						It("is not successful", func() {
							Expect(taskStep.Succeeded()).To(BeFalse())
						})
					})
				})

				Context("when the process exits nonzero", func() {
					BeforeEach(func() {
						fakeProcess.WaitReturns(1, nil)
					})

					It("finishes the task via the delegate", func() {
						Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
						_, status := fakeDelegate.FinishedArgsForCall(0)
						Expect(status).To(Equal(exec.ExitStatus(1)))
					})

					It("saves the exit status property", func() {
						Expect(stepErr).ToNot(HaveOccurred())

						Expect(fakeContainer.SetPropertyCallCount()).To(Equal(1))

						name, value := fakeContainer.SetPropertyArgsForCall(0)
						Expect(name).To(Equal("concourse:exit-status"))
						Expect(value).To(Equal("1"))
					})

					It("is not successful", func() {
						Expect(stepErr).ToNot(HaveOccurred())

						Expect(taskStep.Succeeded()).To(BeFalse())
					})

					Context("when saving the exit status succeeds", func() {
						BeforeEach(func() {
							fakeContainer.SetPropertyReturns(nil)
						})

						It("returns successfully", func() {
							Expect(stepErr).ToNot(HaveOccurred())
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

						It("returns the error", func() {
							Expect(stepErr).To(Equal(disaster))
						})

						It("is not successful", func() {
							Expect(taskStep.Succeeded()).To(BeFalse())
						})
					})
				})

				Context("when waiting on the process fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeProcess.WaitReturns(0, disaster)
					})

					It("returns the error", func() {
						Expect(stepErr).To(Equal(disaster))
					})

					It("is not successful", func() {
						Expect(taskStep.Succeeded()).To(BeFalse())
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

						cancel()
					})

					It("stops the container", func() {
						Expect(fakeContainer.StopCallCount()).To(Equal(1))
						Expect(fakeContainer.StopArgsForCall(0)).To(BeFalse())
						Expect(stepErr).To(Equal(context.Canceled))
					})

					It("is not successful", func() {
						Expect(taskStep.Succeeded()).To(BeFalse())
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
							Expect(stepErr).To(Equal(context.Canceled))
						})

						It("is not successful", func() {
							Expect(taskStep.Succeeded()).To(BeFalse())
						})
					})

					It("doesn't register a source", func() {
						sourceMap := repo.AsMap()
						Expect(sourceMap).To(BeEmpty())
					})
				})

				Context("when running the task's script fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeContainer.RunReturns(nil, disaster)
					})

					It("returns the error", func() {
						Expect(stepErr).To(Equal(disaster))
					})

					It("is not successful", func() {
						Expect(taskStep.Succeeded()).To(BeFalse())
					})
				})
			})
		})

		Context("when creating the container fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeWorkerClient.FindOrCreateContainerReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(stepErr).To(Equal(disaster))
			})

			It("is not successful", func() {
				Expect(taskStep.Succeeded()).To(BeFalse())
			})
		})

		Context("when getting the config fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				configSource.FetchConfigReturns(atc.TaskConfig{}, disaster)
			})

			It("returns the error", func() {
				Expect(stepErr).To(Equal(disaster))
			})

			It("is not successful", func() {
				Expect(taskStep.Succeeded()).To(BeFalse())
			})
		})
	})
})
