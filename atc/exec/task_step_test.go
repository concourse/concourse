package exec_test

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"strings"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/DataDog/zstd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/artifact"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
)

var _ = Describe("TaskStep", func() {
	var (
		ctx    context.Context
		cancel func()
		logger *lagertest.TestLogger

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		fakeClient   *workerfakes.FakeClient
		fakeStrategy *workerfakes.FakeContainerPlacementStrategy

		fakeLockFactory *lockfakes.FakeLockFactory

		fakeSecretManager *credsfakes.FakeSecrets
		fakeDelegate      *execfakes.FakeTaskDelegate
		taskPlan          *atc.TaskPlan

		interpolatedResourceTypes atc.VersionedResourceTypes

		repo  *artifact.Repository
		state *execfakes.FakeRunState

		taskStep exec.Step
		stepErr  error

		containerMetadata = db.ContainerMetadata{
			WorkingDirectory: "some-artifact-root",
			Type:             db.ContainerTypeTask,
			StepName:         "some-step",
		}

		stepMetadata = exec.StepMetadata{
			TeamID:  123,
			BuildID: 1234,
			JobID:   12345,
		}

		planID = atc.PlanID(42)
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		logger = lagertest.NewTestLogger("task-action-test")

		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()

		fakeClient = new(workerfakes.FakeClient)
		fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)

		fakeLockFactory = new(lockfakes.FakeLockFactory)

		fakeSecretManager = new(credsfakes.FakeSecrets)
		fakeSecretManager.GetReturns("super-secret-source", nil, true, nil)

		fakeDelegate = new(execfakes.FakeTaskDelegate)
		fakeDelegate.StdoutReturns(stdoutBuf)
		fakeDelegate.StderrReturns(stderrBuf)

		repo = artifact.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactsReturns(repo)

		uninterpolatedResourceTypes := atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-resource",
					Type:   "custom-type",
					Source: atc.Source{"some-custom": "((source-param))"},
					Params: atc.Params{"some-custom": "param"},
				},
				Version: atc.Version{"some-custom": "version"},
			},
		}

		interpolatedResourceTypes = atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-resource",
					Type:   "custom-type",
					Source: atc.Source{"some-custom": "super-secret-source"},
					Params: atc.Params{"some-custom": "param"},
				},
				Version: atc.Version{"some-custom": "version"},
			},
		}

		taskPlan = &atc.TaskPlan{
			Name:                   "some-task",
			Privileged:             false,
			Tags:                   []string{"step", "tags"},
			VersionedResourceTypes: uninterpolatedResourceTypes,
		}
	})

	JustBeforeEach(func() {
		plan := atc.Plan{
			ID:   atc.PlanID(planID),
			Task: taskPlan,
		}

		// stuff stored on task step still
		taskStep = exec.NewTaskStep(
			plan.ID,
			*plan.Task,
			atc.ContainerLimits{},
			stepMetadata,
			containerMetadata,
			fakeSecretManager,
			fakeStrategy,
			fakeClient,
			fakeDelegate,
			fakeLockFactory,
		)

		stepErr = taskStep.Run(ctx, state)
	})

	Context("when the plan has a config", func() {

		BeforeEach(func() {
			cpu := uint64(1024)
			memory := uint64(1024)

			taskPlan.Config = &atc.TaskConfig{
				Platform: "some-platform",
				ImageResource: &atc.ImageResource{
					Type:    "docker",
					Source:  atc.Source{"some": "secret-source-param"},
					Params:  &atc.Params{"some": "params"},
					Version: &atc.Version{"some": "version"},
				},
				Limits: atc.ContainerLimits{
					CPU:    &cpu,
					Memory: &memory,
				},
				Params: atc.TaskEnv{
					"SECURE": "secret-task-param",
				},
				Run: atc.TaskRunConfig{
					Path: "ls",
					Args: []string{"some", "args"},
				},
			}
		})

		Context("before running the task container", func() {
			BeforeEach(func() {
				fakeDelegate.InitializingStub = func(lager.Logger, atc.TaskConfig) {
					defer GinkgoRecover()
					Expect(fakeClient.RunTaskStepCallCount()).To(BeZero())
				}
			})

			It("invoked the delegate's Initializing callback", func() {
				Expect(fakeDelegate.InitializingCallCount()).To(Equal(1))
			})

			Context("when rootfs uri is set instead of image resource", func() {
				BeforeEach(func() {
					taskPlan.Config = &atc.TaskConfig{
						Platform:  "some-platform",
						RootfsURI: "some-image",
						Params:    map[string]string{"SOME": "params"},
						Run: atc.TaskRunConfig{
							Path: "ls",
							Args: []string{"some", "args"},
						},
					}
				})

				It("correctly sets up the image spec", func() {
					Expect(fakeClient.RunTaskStepCallCount()).To(Equal(1))
					_, _, _, _, containerSpec, _, _, _, _, _, _ := fakeClient.RunTaskStepArgsForCall(0)

					Expect(containerSpec).To(Equal(worker.ContainerSpec{
						Platform: "some-platform",
						Tags:     []string{"step", "tags"},
						TeamID:   stepMetadata.TeamID,
						ImageSpec: worker.ImageSpec{
							ImageURL:   "some-image",
							Privileged: false,
						},
						Type:    "task",
						Dir:     "some-artifact-root",
						Env:     []string{"SOME=params"},
						Inputs:  []worker.InputSource{},
						Outputs: worker.OutputPaths{},
					}))

				})
			})

		})

		It("creates a containerSpec with the correct parameters", func() {
			Expect(fakeClient.RunTaskStepCallCount()).To(Equal(1))

			_, _, _, _, containerSpec, _, _, _, _, _, _ := fakeClient.RunTaskStepArgsForCall(0)

			Expect(containerSpec.Dir).To(Equal("some-artifact-root"))
			Expect(containerSpec.User).To(BeEmpty())
		})

		It("creates the task process spec with the correct parameters", func() {
			Expect(fakeClient.RunTaskStepCallCount()).To(Equal(1))

			_, _, _, _, _, _, _, _, _, taskProcessSpec, _ := fakeClient.RunTaskStepArgsForCall(0)
			Expect(taskProcessSpec.StdoutWriter).To(Equal(stdoutBuf))
			Expect(taskProcessSpec.StderrWriter).To(Equal(stderrBuf))
			Expect(taskProcessSpec.Path).To(Equal("ls"))
			Expect(taskProcessSpec.Args).To(Equal([]string{"some", "args"}))
		})

		Context("when privileged", func() {
			BeforeEach(func() {
				taskPlan.Privileged = true
			})

			It("marks the container's image spec as privileged", func() {
				Expect(fakeClient.RunTaskStepCallCount()).To(Equal(1))
				_, _, _, _, containerSpec, _, _, _, _, _, _ := fakeClient.RunTaskStepArgsForCall(0)
				Expect(containerSpec.ImageSpec.Privileged).To(BeTrue())
			})
		})

		Context("when the configuration specifies paths for inputs", func() {
			var inputSource *workerfakes.FakeArtifactSource
			var otherInputSource *workerfakes.FakeArtifactSource

			BeforeEach(func() {
				inputSource = new(workerfakes.FakeArtifactSource)
				otherInputSource = new(workerfakes.FakeArtifactSource)

				taskPlan.Config = &atc.TaskConfig{
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
				}
			})

			Context("when all inputs are present", func() {
				BeforeEach(func() {
					repo.RegisterSource("some-input", inputSource)
					repo.RegisterSource("some-other-input", otherInputSource)
				})

				It("configures the inputs for the containerSpec correctly", func() {
					Expect(fakeClient.RunTaskStepCallCount()).To(Equal(1))
					_, _, _, _, containerSpec, _, _, _, _, _, _ := fakeClient.RunTaskStepArgsForCall(0)
					Expect(containerSpec.Inputs).To(HaveLen(2))
					for _, input := range containerSpec.Inputs {
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
				taskPlan.InputMapping = map[string]string{"remapped-input": "remapped-input-src"}
				taskPlan.Config = &atc.TaskConfig{
					Platform: "some-platform",
					Run: atc.TaskRunConfig{
						Path: "ls",
						Args: []string{"some", "args"},
					},
					Inputs: []atc.TaskInputConfig{
						{Name: "remapped-input"},
					},
				}
			})

			Context("when all inputs are present in the in source repository", func() {
				BeforeEach(func() {
					repo.RegisterSource("remapped-input-src", remappedInputSource)
				})

				It("uses remapped input", func() {
					Expect(fakeClient.RunTaskStepCallCount()).To(Equal(1))
					_, _, _, _, containerSpec, _, _, _, _, _, _ := fakeClient.RunTaskStepArgsForCall(0)
					Expect(containerSpec.Inputs).To(HaveLen(1))
					Expect(containerSpec.Inputs[0].Source()).To(Equal(remappedInputSource))
					Expect(containerSpec.Inputs[0].DestinationPath()).To(Equal("some-artifact-root/remapped-input"))
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
				taskPlan.Config = &atc.TaskConfig{
					Platform: "some-platform",
					Run: atc.TaskRunConfig{
						Path: "ls",
					},
					Inputs: []atc.TaskInputConfig{
						{Name: "optional-input", Optional: true},
						{Name: "optional-input-2", Optional: true},
						{Name: "required-input"},
					},
				}
			})

			Context("when an optional input is missing", func() {
				BeforeEach(func() {
					repo.RegisterSource("required-input", requiredInputSource)
					repo.RegisterSource("optional-input-2", optionalInput2Source)
				})

				It("runs successfully without the optional input", func() {
					Expect(stepErr).ToNot(HaveOccurred())
					Expect(fakeClient.RunTaskStepCallCount()).To(Equal(1))
					_, _, _, _, containerSpec, _, _, _, _, _, _ := fakeClient.RunTaskStepArgsForCall(0)
					Expect(containerSpec.Inputs).To(HaveLen(2))
					Expect(containerSpec.Inputs[0].Source()).To(Equal(optionalInput2Source))
					Expect(containerSpec.Inputs[0].DestinationPath()).To(Equal("some-artifact-root/optional-input-2"))
					Expect(containerSpec.Inputs[1].Source()).To(Equal(requiredInputSource))
					Expect(containerSpec.Inputs[1].DestinationPath()).To(Equal("some-artifact-root/required-input"))
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
				taskPlan.Config = &atc.TaskConfig{
					Platform:  "some-platform",
					RootfsURI: "some-image",
					Run: atc.TaskRunConfig{
						Path: "ls",
					},
					Caches: []atc.TaskCacheConfig{
						{Path: "some-path-1"},
						{Path: "some-path-2"},
					},
				}

				fakeVolume1 = new(workerfakes.FakeVolume)
				fakeVolume2 = new(workerfakes.FakeVolume)
				taskResult := worker.TaskResult{
					Status: 0,
					VolumeMounts: []worker.VolumeMount{
						worker.VolumeMount{
							Volume:    fakeVolume1,
							MountPath: "some-artifact-root/some-path-1",
						},
						worker.VolumeMount{
							Volume:    fakeVolume2,
							MountPath: "some-artifact-root/some-path-2",
						},
					},
					Err: nil,
				}
				fakeClient.RunTaskStepReturns(taskResult)
			})

			It("creates the containerSpec with the caches in the inputs", func() {
				_, _, _, _, containerSpec, _, _, _, _, _, _ := fakeClient.RunTaskStepArgsForCall(0)
				Expect(containerSpec.Inputs).To(HaveLen(2))
				Expect([]string{
					containerSpec.Inputs[0].DestinationPath(),
					containerSpec.Inputs[1].DestinationPath(),
				}).To(ConsistOf(
					"some-artifact-root/some-path-1",
					"some-artifact-root/some-path-2",
				))
			})

			Context("when task belongs to a job", func() {
				BeforeEach(func() {
					stepMetadata.JobID = 12
				})

				It("registers cache volumes as task caches", func() {
					Expect(stepErr).ToNot(HaveOccurred())

					Expect(fakeVolume1.InitializeTaskCacheCallCount()).To(Equal(1))
					_, jID, stepName, cachePath, p := fakeVolume1.InitializeTaskCacheArgsForCall(0)
					Expect(jID).To(Equal(stepMetadata.JobID))
					Expect(stepName).To(Equal("some-task"))
					Expect(cachePath).To(Equal("some-path-1"))
					Expect(p).To(Equal(bool(taskPlan.Privileged)))

					Expect(fakeVolume2.InitializeTaskCacheCallCount()).To(Equal(1))
					_, jID, stepName, cachePath, p = fakeVolume2.InitializeTaskCacheArgsForCall(0)
					Expect(jID).To(Equal(stepMetadata.JobID))
					Expect(stepName).To(Equal("some-task"))
					Expect(cachePath).To(Equal("some-path-2"))
					Expect(p).To(Equal(bool(taskPlan.Privileged)))
				})
			})

			Context("when task does not belong to job (one-off build)", func() {
				BeforeEach(func() {
					stepMetadata.JobID = 0
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
				taskPlan.Config = &atc.TaskConfig{
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
				}
			})

			It("configures them appropriately in the container spec", func() {
				_, _, _, _, containerSpec, _, _, _, _, _, _ := fakeClient.RunTaskStepArgsForCall(0)
				Expect(containerSpec.Outputs).To(Equal(worker.OutputPaths{
					"some-output":                "some-artifact-root/some-output-configured-path/",
					"some-other-output":          "some-artifact-root/some-other-output/",
					"some-trailing-slash-output": "some-artifact-root/some-output-configured-path-with-trailing-slash/",
				}))
			})
		})

		Context("when missing the platform", func() {

			BeforeEach(func() {
				taskPlan.Config.Platform = ""
			})

			It("returns the error", func() {
				Expect(stepErr).To(HaveOccurred())
			})

			It("is not successful", func() {
				Expect(taskStep.Succeeded()).To(BeFalse())
			})
		})

		Context("when missing the path to the executable", func() {

			BeforeEach(func() {
				taskPlan.Config.Run.Path = ""
			})

			It("returns the error", func() {
				Expect(stepErr).To(HaveOccurred())
			})

			It("is not successful", func() {
				Expect(taskStep.Succeeded()).To(BeFalse())
			})
		})

		Context("when an image artifact name is specified", func() {
			BeforeEach(func() {
				taskPlan.ImageArtifactName = "some-image-artifact"
			})

			Context("when the image artifact is registered in the source repo", func() {
				var imageArtifactSource *workerfakes.FakeArtifactSource

				BeforeEach(func() {
					imageArtifactSource = new(workerfakes.FakeArtifactSource)
					repo.RegisterSource("some-image-artifact", imageArtifactSource)
				})

				It("configures it in the containerSpec's ImageSpec", func() {
					_, _, _, _, containerSpec, workerSpec, _, _, _, _, _ := fakeClient.RunTaskStepArgsForCall(0)
					Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
						ImageArtifactSource: imageArtifactSource,
					}))

					Expect(workerSpec.ResourceType).To(Equal(""))
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
								taskPlan.Config = &atc.TaskConfig{
									Platform:  "some-platform",
									RootfsURI: "some-image",
									Params:    map[string]string{"SOME": "params"},
									Run: atc.TaskRunConfig{
										Path: "ls",
										Args: []string{"some", "args"},
									},
								}
							})

							It("still uses the image artifact source", func() {
								_, _, _, _, containerSpec, workerSpec, _, _, _, _, _ := fakeClient.RunTaskStepArgsForCall(0)
								Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
									ImageArtifactSource: imageArtifactSource,
								}))

								Expect(workerSpec.ResourceType).To(Equal(""))
							})
						})

						Context("when the task config also specifies image_resource", func() {
							BeforeEach(func() {
								taskPlan.Config = &atc.TaskConfig{
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
							})

							It("still uses the image artifact source", func() {
								_, _, _, _, containerSpec, workerSpec, _, _, _, _, _ := fakeClient.RunTaskStepArgsForCall(0)
								Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
									ImageArtifactSource: imageArtifactSource,
								}))

								Expect(workerSpec.ResourceType).To(Equal(""))
							})
						})

						Context("when the task config also specifies image and image_resource", func() {
							BeforeEach(func() {
								taskPlan.Config = &atc.TaskConfig{
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
							})

							It("still uses the image artifact source", func() {
								_, _, _, _, containerSpec, workerSpec, _, _, _, _, _ := fakeClient.RunTaskStepArgsForCall(0)
								Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
									ImageArtifactSource: imageArtifactSource,
								}))
								Expect(workerSpec.ResourceType).To(Equal(""))
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

		Context("when the image_resource is specified (even if RootfsURI is configured)", func() {
			BeforeEach(func() {
				taskPlan.Config = &atc.TaskConfig{
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
			})

			It("creates the specs with the image resource", func() {
				_, _, _, _, containerSpec, workerSpec, _, _, _, _, _ := fakeClient.RunTaskStepArgsForCall(0)
				Expect(containerSpec.ImageSpec.ImageResource).To(Equal(&worker.ImageResource{
					Type:    "docker",
					Source:  atc.Source{"some": "super-secret-source"},
					Params:  &atc.Params{"some": "params"},
					Version: &atc.Version{"some": "version"},
				}))

				Expect(workerSpec).To(Equal(worker.WorkerSpec{
					TeamID:        123,
					Platform:      "some-platform",
					ResourceTypes: interpolatedResourceTypes,
					Tags:          []string{"step", "tags"},
					ResourceType:  "docker",
				}))
			})
		})

		Context("when the RootfsURI is configured", func() {
			BeforeEach(func() {
				taskPlan.Config = &atc.TaskConfig{
					Platform:  "some-platform",
					RootfsURI: "some-image",
					Params:    map[string]string{"SOME": "params"},
					Run: atc.TaskRunConfig{
						Path: "ls",
						Args: []string{"some", "args"},
					},
				}
			})

			It("creates the specs with the image resource", func() {
				_, _, _, _, containerSpec, workerSpec, _, _, _, _, _ := fakeClient.RunTaskStepArgsForCall(0)
				Expect(containerSpec.ImageSpec.ImageURL).To(Equal("some-image"))

				Expect(workerSpec).To(Equal(worker.WorkerSpec{
					TeamID:        123,
					Platform:      "some-platform",
					ResourceTypes: interpolatedResourceTypes,
					Tags:          []string{"step", "tags"},
				}))
			})
		})

		Context("when a run dir is specified", func() {
			var dir string
			BeforeEach(func() {
				dir = "/some/dir"
				taskPlan.Config.Run.Dir = dir
			})

			It("specifies it in the process  spec", func() {
				_, _, _, _, _, _, _, _, _, processSpec, _ := fakeClient.RunTaskStepArgsForCall(0)
				Expect(processSpec.Dir).To(Equal(dir))
			})
		})

		Context("when a run user is specified", func() {
			BeforeEach(func() {
				taskPlan.Config.Run.User = "some-user"
			})

			It("adds the user to the container spec", func() {
				_, _, _, _, containerSpec, _, _, _, _, _, _ := fakeClient.RunTaskStepArgsForCall(0)
				Expect(containerSpec.User).To(Equal("some-user"))
			})

			It("doesn't bother adding the user to the run spec", func() {
				_, _, _, _, _, _, _, _, _, processSpec, _ := fakeClient.RunTaskStepArgsForCall(0)
				Expect(processSpec.User).To(BeEmpty())
			})
		})

		Context("when running the task succeeds", func() {
			var taskStepStatus int
			BeforeEach(func() {
				taskPlan.Config = &atc.TaskConfig{
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
				}
			})

			It("returns successfully", func() {
				Expect(stepErr).ToNot(HaveOccurred())
			})

			Context("when the task exits with zero status", func() {
				BeforeEach(func() {
					taskStepStatus = 0
					taskResult := worker.TaskResult{
						Status:       taskStepStatus,
						VolumeMounts: []worker.VolumeMount{},
						Err:          nil,
					}
					fakeClient.RunTaskStepReturns(taskResult)
				})
				It("finishes the task via the delegate", func() {
					Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
					_, status := fakeDelegate.FinishedArgsForCall(0)
					Expect(status).To(Equal(exec.ExitStatus(taskStepStatus)))
				})

				It("returns successfully", func() {
					Expect(stepErr).ToNot(HaveOccurred())
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

						fakeTaskResult := worker.TaskResult{
							Status: 0,
							VolumeMounts: []worker.VolumeMount{
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
							},
							Err: nil,
						}
						fakeClient.RunTaskStepReturns(fakeTaskResult)
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
							_, _, _, _, containerSpec, _, _, _, _, _, _ := fakeClient.RunTaskStepArgsForCall(0)
							Expect(containerSpec.Outputs).To(Equal(worker.OutputPaths{
								"some-output":                "some-artifact-root/some-output-configured-path/",
								"some-other-output":          "some-artifact-root/some-other-output/",
								"some-trailing-slash-output": "some-artifact-root/some-output-configured-path-with-trailing-slash/",
							}))
						})

						It("streams the data from the volumes to the destination", func() {
							err := artifactSource1.StreamTo(logger, fakeDestination)
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
									zstdWriter := zstd.NewWriter(tgzBuffer)
									defer zstdWriter.Close()

									tarWriter := tar.NewWriter(zstdWriter)
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
									reader, err := artifactSource1.StreamFile(logger, "some-path")
									Expect(err).NotTo(HaveOccurred())

									Expect(ioutil.ReadAll(reader)).To(Equal([]byte(fileContent)))

									path := fakeVolume1.StreamOutArgsForCall(0)
									Expect(path).To(Equal("some-path"))
								})

								Describe("closing the stream", func() {
									It("closes the stream from the versioned source", func() {
										reader, err := artifactSource1.StreamFile(logger, "some-path")
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
									_, err := artifactSource1.StreamFile(logger, "some-path")
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
								_, err := artifactSource1.StreamFile(logger, "some-path")
								Expect(err).To(Equal(disaster))
							})
						})
					})
				})
			})

			Context("when the task exits with nonzero status", func() {
				BeforeEach(func() {
					taskStepStatus = 5
					taskResult := worker.TaskResult{Status: taskStepStatus, VolumeMounts: []worker.VolumeMount{}, Err: nil}
					fakeClient.RunTaskStepReturns(taskResult)
				})
				It("finishes the task via the delegate", func() {
					Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
					_, status := fakeDelegate.FinishedArgsForCall(0)
					Expect(status).To(Equal(exec.ExitStatus(taskStepStatus)))
				})

				It("returns successfully", func() {
					Expect(stepErr).ToNot(HaveOccurred())
				})
			})
		})

		Context("when running the task fails", func() {
			disaster := errors.New("task run failed")

			BeforeEach(func() {
				taskResult := worker.TaskResult{Status: -1, VolumeMounts: []worker.VolumeMount{}, Err: disaster}
				fakeClient.RunTaskStepReturns(taskResult)
			})

			It("returns the error", func() {
				Expect(stepErr).To(Equal(disaster))
			})

			It("is not successful", func() {
				Expect(taskStep.Succeeded()).To(BeFalse())
			})
		})

		Context("when the task step is interrupted", func() {
			BeforeEach(func() {
				fakeClient.RunTaskStepReturns(
					worker.TaskResult{
						Status:       -1,
						VolumeMounts: []worker.VolumeMount{},
						Err:          context.Canceled,
					},
				)
				cancel()
			})

			It("returns the context.Canceled error", func() {
				Expect(stepErr).To(Equal(context.Canceled))
			})

			It("is not successful", func() {
				Expect(taskStep.Succeeded()).To(BeFalse())
			})

			It("waits for RunTaskStep to return", func() {
				Expect(fakeClient.RunTaskStepCallCount()).To(Equal(1))
			})

			It("doesn't register a source", func() {
				sourceMap := repo.AsMap()
				Expect(sourceMap).To(BeEmpty())
			})
		})

		Context("when RunTaskStep returns volume mounts", func() {
			var (
				fakeMountPath1 string = "some-artifact-root/some-output-configured-path/"
				fakeMountPath2 string = "some-artifact-root/some-other-output/"
				fakeMountPath3 string = "some-artifact-root/some-output-configured-path-with-trailing-slash/"

				fakeVolume1 *workerfakes.FakeVolume
				fakeVolume2 *workerfakes.FakeVolume
				fakeVolume3 *workerfakes.FakeVolume
			)

			BeforeEach(func() {
				taskPlan.Config = &atc.TaskConfig{
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
				}

				fakeVolume1 = new(workerfakes.FakeVolume)
				fakeVolume1.HandleReturns("some-handle-1")
				fakeVolume2 = new(workerfakes.FakeVolume)
				fakeVolume2.HandleReturns("some-handle-2")
				fakeVolume3 = new(workerfakes.FakeVolume)
				fakeVolume3.HandleReturns("some-handle-3")

				taskResult := worker.TaskResult{
					Status: 0,
					VolumeMounts: []worker.VolumeMount{
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
					},
					Err: nil,
				}
				fakeClient.RunTaskStepReturns(taskResult)
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

		Context("when output is remapped", func() {
			var (
				fakeMountPath string = "some-artifact-root/generic-remapped-output/"
			)

			BeforeEach(func() {
				taskPlan.OutputMapping = map[string]string{"generic-remapped-output": "specific-remapped-output"}
				taskPlan.Config = &atc.TaskConfig{
					Platform: "some-platform",
					Run: atc.TaskRunConfig{
						Path: "ls",
					},
					Outputs: []atc.TaskOutputConfig{
						{Name: "generic-remapped-output"},
					},
				}

				fakeVolume := new(workerfakes.FakeVolume)
				fakeVolume.HandleReturns("some-handle")

				taskResult := worker.TaskResult{
					Status: 0,
					VolumeMounts: []worker.VolumeMount{
						worker.VolumeMount{
							Volume:    fakeVolume,
							MountPath: fakeMountPath,
						},
					},
					Err: nil,
				}
				fakeClient.RunTaskStepReturns(taskResult)
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

	})
})
