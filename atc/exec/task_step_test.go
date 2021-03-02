package exec_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/runtime/runtimefakes"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/vars"
	"github.com/onsi/gomega/gbytes"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/api/trace/tracetest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TaskStep", func() {
	var (
		ctx    context.Context
		cancel func()

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		fakePool             *workerfakes.FakePool
		fakeClient           *workerfakes.FakeClient
		fakeArtifactStreamer *workerfakes.FakeArtifactStreamer
		fakeArtifactSourcer  *workerfakes.FakeArtifactSourcer
		fakeStrategy         *workerfakes.FakeContainerPlacementStrategy

		spanCtx      context.Context
		fakeDelegate *execfakes.FakeTaskDelegate

		fakeDelegateFactory *execfakes.FakeTaskDelegateFactory

		taskPlan *atc.TaskPlan

		repo       *build.Repository
		state      *execfakes.FakeRunState
		childState *execfakes.FakeRunState

		taskStep exec.Step
		stepOk   bool
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

		planID = atc.PlanID("42")

		shouldRunTaskStep bool
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()

		fakeClient = new(workerfakes.FakeClient)
		fakeClient.NameReturns("some-worker")
		fakePool = new(workerfakes.FakePool)
		fakePool.WaitForWorkerReturns(fakeClient, 0, nil)

		fakeArtifactStreamer = new(workerfakes.FakeArtifactStreamer)
		fakeArtifactSourcer = new(workerfakes.FakeArtifactSourcer)
		fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)

		fakeDelegate = new(execfakes.FakeTaskDelegate)
		fakeDelegate.StdoutReturns(stdoutBuf)
		fakeDelegate.StderrReturns(stderrBuf)

		spanCtx = context.Background()
		fakeDelegate.StartSpanReturns(spanCtx, trace.NoopSpan{})

		fakeDelegateFactory = new(execfakes.FakeTaskDelegateFactory)
		fakeDelegateFactory.TaskDelegateReturns(fakeDelegate)

		repo = build.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactRepositoryReturns(repo)

		childState = new(execfakes.FakeRunState)
		childState.ArtifactRepositoryReturns(repo.NewLocalScope())
		state.NewLocalScopeReturns(childState)

		state.GetStub = vars.StaticVariables{"source-param": "super-secret-source"}.Get

		taskPlan = &atc.TaskPlan{
			Name:       "some-task",
			Privileged: false,
			VersionedResourceTypes: atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Name:   "custom-resource",
						Type:   "custom-type",
						Source: atc.Source{"some-custom": "((source-param))"},
						Params: atc.Params{"some-custom": "param"},
					},
					Version: atc.Version{"some-custom": "version"},
				},
			},
		}

		shouldRunTaskStep = true
	})

	JustBeforeEach(func() {
		plan := atc.Plan{
			ID:   planID,
			Task: taskPlan,
		}

		// stuff stored on task step still
		taskStep = exec.NewTaskStep(
			plan.ID,
			*plan.Task,
			atc.ContainerLimits{},
			stepMetadata,
			containerMetadata,
			fakeStrategy,
			fakePool,
			fakeArtifactStreamer,
			fakeArtifactSourcer,
			fakeDelegateFactory,
		)

		stepOk, stepErr = taskStep.Run(ctx, state)
	})

	var runCtx context.Context
	var owner db.ContainerOwner
	var containerSpec worker.ContainerSpec
	var metadata db.ContainerMetadata
	var processSpec runtime.ProcessSpec
	var startEventDelegate runtime.StartingEventDelegate

	expectWorkerSpecResourceTypeUnset := func() {
		Expect(fakePool.WaitForWorkerCallCount()).To(Equal(1))
		_, _, _, workerSpec, _ := fakePool.WaitForWorkerArgsForCall(0)
		Expect(workerSpec.ResourceType).To(Equal(""))
	}

	JustBeforeEach(func() {
		if shouldRunTaskStep {
			Expect(fakeClient.RunTaskStepCallCount()).To(Equal(1), "task step should have run")
			runCtx, owner, containerSpec, metadata, processSpec, startEventDelegate = fakeClient.RunTaskStepArgsForCall(0)
		} else {
			Expect(fakeClient.RunTaskStepCallCount()).To(Equal(0), "task step should NOT have run")
		}
	})

	Context("when the plan has a config", func() {
		BeforeEach(func() {
			cpu := atc.CPULimit(1024)
			memory := atc.MemoryLimit(1024)

			taskPlan.Config = &atc.TaskConfig{
				Platform: "some-platform",
				Limits: &atc.ContainerLimits{
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
				fakeDelegate.InitializingStub = func(lager.Logger) {
					defer GinkgoRecover()
					Expect(fakeClient.RunTaskStepCallCount()).To(BeZero())
				}
			})

			It("invokes the delegate's Initializing callback", func() {
				Expect(fakeDelegate.InitializingCallCount()).To(Equal(1))
			})
		})

		Describe("worker selection", func() {
			var workerSpec worker.WorkerSpec

			JustBeforeEach(func() {
				Expect(fakePool.WaitForWorkerCallCount()).To(Equal(1))
				_, _, _, workerSpec, _ = fakePool.WaitForWorkerArgsForCall(0)
			})

			It("emits a SelectedWorker event", func() {
				Expect(fakeDelegate.SelectedWorkerCallCount()).To(Equal(1))
				_, workerName := fakeDelegate.SelectedWorkerArgsForCall(0)
				Expect(workerName).To(Equal("some-worker"))
			})

			Context("when tags are configured", func() {
				BeforeEach(func() {
					taskPlan.Tags = atc.Tags{"plan", "tags"}
				})

				It("creates a worker spec with the tags", func() {
					Expect(workerSpec.Tags).To(Equal([]string{"plan", "tags"}))
				})
			})

			Context("when selecting a worker fails", func() {
				BeforeEach(func() {
					fakePool.WaitForWorkerReturns(nil, 0, errors.New("nope"))
					shouldRunTaskStep = false
				})

				It("returns an err", func() {
					Expect(stepErr).To(MatchError(ContainSubstring("nope")))
				})
			})
		})

		It("creates a containerSpec with the correct parameters", func() {
			Expect(containerSpec.Dir).To(Equal("some-artifact-root"))
			Expect(containerSpec.User).To(BeEmpty())
		})

		It("creates the task process spec with the correct parameters", func() {
			Expect(processSpec.StdoutWriter).To(Equal(stdoutBuf))
			Expect(processSpec.StderrWriter).To(Equal(stderrBuf))
			Expect(processSpec.Path).To(Equal("ls"))
			Expect(processSpec.Args).To(Equal([]string{"some", "args"}))
		})

		It("sets the config on the TaskDelegate", func() {
			Expect(fakeDelegate.SetTaskConfigCallCount()).To(Equal(1))
			actualTaskConfig := fakeDelegate.SetTaskConfigArgsForCall(0)
			Expect(actualTaskConfig).To(Equal(*taskPlan.Config))
		})

		It("uses the correct owner", func() {
			Expect(owner).To(Equal(db.NewBuildStepContainerOwner(1234, atc.PlanID(planID), 123)))
		})

		It("uses the correct metadata", func() {
			Expect(metadata).To(Equal(containerMetadata))
		})

		It("uses the correct delegate", func() {
			Expect(startEventDelegate).To(Equal(fakeDelegate))
		})

		Context("when privileged", func() {
			BeforeEach(func() {
				taskPlan.Privileged = true
			})

			It("marks the container's image spec as privileged", func() {
				Expect(containerSpec.ImageSpec.Privileged).To(BeTrue())
			})
		})

		Context("when a timeout is configured", func() {
			BeforeEach(func() {
				taskPlan.Timeout = "1h"
			})

			It("enforces it on the context", func() {
				t, ok := runCtx.Deadline()
				Expect(ok).To(BeTrue())
				Expect(t).To(BeTemporally("~", time.Now().Add(time.Hour), time.Minute))
			})

			Context("when running times out", func() {
				BeforeEach(func() {
					fakeClient.RunTaskStepReturns(
						worker.TaskResult{},
						fmt.Errorf("wrapped: %w", context.DeadlineExceeded),
					)
				})

				It("fails without error", func() {
					Expect(stepOk).To(BeFalse())
					Expect(stepErr).To(BeNil())
				})

				It("emits an Errored event", func() {
					Expect(fakeDelegate.ErroredCallCount()).To(Equal(1))
					_, status := fakeDelegate.ErroredArgsForCall(0)
					Expect(status).To(Equal(exec.TimeoutLogMessage))
				})
			})

			Context("when the timeout is bogus", func() {
				BeforeEach(func() {
					taskPlan.Timeout = "bogus"
					shouldRunTaskStep = false
				})

				It("fails miserably", func() {
					Expect(stepErr).To(MatchError("parse timeout: time: invalid duration \"bogus\""))
				})
			})
		})

		Context("when rootfs uri is set instead of image resource", func() {
			BeforeEach(func() {
				taskPlan.Config.RootfsURI = "some-image"
			})

			It("correctly sets up the image spec", func() {
				Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
					ImageURL:   "some-image",
					Privileged: false,
				}))
			})
		})

		Context("when tracing is enabled", func() {
			var buildSpan trace.Span

			BeforeEach(func() {
				tracing.ConfigureTraceProvider(tracetest.NewProvider())

				spanCtx, buildSpan = tracing.StartSpan(ctx, "build", nil)
				fakeDelegate.StartSpanReturns(spanCtx, buildSpan)
			})

			AfterEach(func() {
				tracing.Configured = false
			})

			It("propagates span context to the worker client", func() {
				Expect(runCtx).To(Equal(rewrapLogger(spanCtx)))
			})

			It("populates the TRACEPARENT env var", func() {
				Expect(containerSpec.Env).To(ContainElement(MatchRegexp(`TRACEPARENT=.+`)))
			})
		})

		Context("when the configuration specifies paths for inputs", func() {
			var inputArtifact *runtimefakes.FakeArtifact
			var otherInputArtifact *runtimefakes.FakeArtifact

			BeforeEach(func() {
				inputArtifact = new(runtimefakes.FakeArtifact)
				otherInputArtifact = new(runtimefakes.FakeArtifact)

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
					repo.RegisterArtifact("some-input", inputArtifact)
					repo.RegisterArtifact("some-other-input", otherInputArtifact)
				})

				It("configures the inputs for the containerSpec correctly", func() {
					Expect(fakeArtifactSourcer.SourceInputsAndCachesCallCount()).To(Equal(1))
					_, teamID, inputMap := fakeArtifactSourcer.SourceInputsAndCachesArgsForCall(0)
					Expect(teamID).To(Equal(123))
					Expect(inputMap).To(HaveLen(2))
					Expect(inputMap["some-artifact-root/some-input-configured-path"]).To(Equal(inputArtifact))
					Expect(inputMap["some-artifact-root/some-other-input"]).To(Equal(otherInputArtifact))
				})
			})

			Context("when any of the inputs are missing", func() {
				BeforeEach(func() {
					repo.RegisterArtifact("some-input", inputArtifact)

					shouldRunTaskStep = false
				})

				It("returns a MissingInputsError", func() {
					Expect(stepErr).To(BeAssignableToTypeOf(exec.MissingInputsError{}))
					Expect(stepErr.(exec.MissingInputsError).Inputs).To(ConsistOf("some-other-input"))
				})
			})
		})

		Context("when input is remapped", func() {
			var remappedInputArtifact *runtimefakes.FakeArtifact

			BeforeEach(func() {
				remappedInputArtifact = new(runtimefakes.FakeArtifact)
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

			Context("when all inputs are present in the in artifact repository", func() {
				BeforeEach(func() {
					repo.RegisterArtifact("remapped-input-src", remappedInputArtifact)
				})

				It("uses remapped input", func() {
					_, _, inputMap := fakeArtifactSourcer.SourceInputsAndCachesArgsForCall(0)
					Expect(inputMap).To(HaveLen(1))
					Expect(inputMap["some-artifact-root/remapped-input"]).To(Equal(remappedInputArtifact))
					Expect(stepErr).ToNot(HaveOccurred())
				})
			})

			Context("when any of the inputs are missing", func() {
				BeforeEach(func() {
					shouldRunTaskStep = false
				})

				It("returns a MissingInputsError", func() {
					Expect(stepErr).To(BeAssignableToTypeOf(exec.MissingInputsError{}))
					Expect(stepErr.(exec.MissingInputsError).Inputs).To(ConsistOf("remapped-input-src"))
				})
			})
		})

		Context("when some inputs are optional", func() {
			var (
				optionalInputArtifact, optionalInput2Artifact, requiredInputArtifact *runtimefakes.FakeArtifact
			)

			BeforeEach(func() {
				optionalInputArtifact = new(runtimefakes.FakeArtifact)
				optionalInput2Artifact = new(runtimefakes.FakeArtifact)
				requiredInputArtifact = new(runtimefakes.FakeArtifact)
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
					repo.RegisterArtifact("required-input", requiredInputArtifact)
					repo.RegisterArtifact("optional-input-2", optionalInput2Artifact)
				})

				It("runs successfully without the optional input", func() {
					Expect(stepErr).ToNot(HaveOccurred())
					_, _, inputMap := fakeArtifactSourcer.SourceInputsAndCachesArgsForCall(0)
					Expect(inputMap).To(HaveLen(2))
					Expect(inputMap["some-artifact-root/required-input"]).To(Equal(optionalInputArtifact))
					Expect(inputMap["some-artifact-root/optional-input-2"]).To(Equal(optionalInput2Artifact))
				})
			})

			Context("when a required input is missing", func() {
				BeforeEach(func() {
					repo.RegisterArtifact("optional-input", optionalInputArtifact)
					repo.RegisterArtifact("optional-input-2", optionalInput2Artifact)

					shouldRunTaskStep = false
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

				taskResult worker.TaskResult
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
				taskResult = worker.TaskResult{
					ExitStatus: 0,
					VolumeMounts: []worker.VolumeMount{
						{
							Volume:    fakeVolume1,
							MountPath: "some-artifact-root/some-path-1",
						},
						{
							Volume:    fakeVolume2,
							MountPath: "some-artifact-root/some-path-2",
						},
					},
				}
				fakeClient.RunTaskStepReturns(taskResult, nil)
			})

			It("creates the containerSpec with the caches in the inputs", func() {
				_, _, inputMap := fakeArtifactSourcer.SourceInputsAndCachesArgsForCall(0)
				Expect(inputMap).To(HaveLen(2))
				Expect(inputMap["some-artifact-root/some-path-1"]).ToNot(BeNil())
				Expect(inputMap["some-artifact-root/some-path-2"]).ToNot(BeNil())
			})

			itRegistersCaches := func() {
				It("registers cache volumes as task caches", func() {
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
			}

			Context("when task belongs to a job", func() {
				BeforeEach(func() {
					stepMetadata.JobID = 12
				})

				Context("when the task succeeds", func() {
					BeforeEach(func() {
						taskResult.ExitStatus = 0
						fakeClient.RunTaskStepReturns(taskResult, nil)
					})

					itRegistersCaches()
				})

				Context("when the task exits nonzero", func() {
					BeforeEach(func() {
						taskResult.ExitStatus = 1
						fakeClient.RunTaskStepReturns(taskResult, nil)
					})

					itRegistersCaches()
				})

				Context("when the task errors", func() {
					BeforeEach(func() {
						taskResult.ExitStatus = 1
						fakeClient.RunTaskStepReturns(taskResult, errors.New("bam"))
					})

					itRegistersCaches()
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

				shouldRunTaskStep = false
			})

			It("returns the error", func() {
				Expect(stepErr).To(HaveOccurred())
			})

			It("is not successful", func() {
				Expect(stepOk).To(BeFalse())
			})
		})

		Context("when missing the path to the executable", func() {

			BeforeEach(func() {
				taskPlan.Config.Run.Path = ""

				shouldRunTaskStep = false
			})

			It("returns the error", func() {
				Expect(stepErr).To(HaveOccurred())
			})

			It("is not successful", func() {
				Expect(stepOk).To(BeFalse())
			})
		})

		Context("when an image artifact name is specified", func() {
			BeforeEach(func() {
				taskPlan.ImageArtifactName = "some-image-artifact"
			})

			Context("when the image artifact is registered in the artifact repo", func() {
				var imageArtifact *runtimefakes.FakeArtifact
				var source *workerfakes.FakeStreamableArtifactSource

				BeforeEach(func() {
					imageArtifact = new(runtimefakes.FakeArtifact)
					repo.RegisterArtifact("some-image-artifact", imageArtifact)

					source = new(workerfakes.FakeStreamableArtifactSource)
					fakeArtifactSourcer.SourceImageReturns(source, nil)
				})

				It("configures it in the containerSpec's ImageSpec", func() {
					Expect(fakeArtifactSourcer.SourceImageCallCount()).To(Equal(1))
					_, artifact := fakeArtifactSourcer.SourceImageArgsForCall(0)
					Expect(artifact).To(Equal(imageArtifact))

					Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
						ImageArtifactSource: source,
					}))

					expectWorkerSpecResourceTypeUnset()
				})

				Describe("when task config specifies image and/or image resource as well as image artifact", func() {
					Context("when streaming the metadata from the worker succeeds", func() {

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

							It("still uses the image artifact", func() {
								_, artifact := fakeArtifactSourcer.SourceImageArgsForCall(0)
								Expect(artifact).To(Equal(imageArtifact))

								expectWorkerSpecResourceTypeUnset()
							})
						})

						Context("when the task config also specifies image_resource", func() {
							BeforeEach(func() {
								taskPlan.Config = &atc.TaskConfig{
									Platform: "some-platform",
									ImageResource: &atc.ImageResource{
										Type:    "docker",
										Source:  atc.Source{"some": "super-secret-source"},
										Params:  atc.Params{"some": "params"},
										Version: atc.Version{"some": "version"},
									},
									Params: map[string]string{"SOME": "params"},
									Run: atc.TaskRunConfig{
										Path: "ls",
										Args: []string{"some", "args"},
									},
								}
							})

							It("still uses the image artifact", func() {
								_, artifact := fakeArtifactSourcer.SourceImageArgsForCall(0)
								Expect(artifact).To(Equal(imageArtifact))

								expectWorkerSpecResourceTypeUnset()
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
										Params:  atc.Params{"some": "params"},
										Version: atc.Version{"some": "version"},
									},
									Params: map[string]string{"SOME": "params"},
									Run: atc.TaskRunConfig{
										Path: "ls",
										Args: []string{"some", "args"},
									},
								}
							})

							It("still uses the image artifact", func() {
								_, artifact := fakeArtifactSourcer.SourceImageArgsForCall(0)
								Expect(artifact).To(Equal(imageArtifact))
								expectWorkerSpecResourceTypeUnset()
							})
						})
					})
				})
			})

			Context("when the image artifact is NOT registered in the artifact repo", func() {
				BeforeEach(func() {
					shouldRunTaskStep = false
				})

				It("returns a MissingTaskImageSourceError", func() {
					Expect(stepErr).To(Equal(exec.MissingTaskImageSourceError{"some-image-artifact"}))
				})

				It("is not successful", func() {
					Expect(stepOk).To(BeFalse())
				})
			})
		})

		Context("when the image_resource is specified (even if RootfsURI is configured)", func() {
			var fakeImageSpec worker.ImageSpec

			BeforeEach(func() {
				taskPlan.Config = &atc.TaskConfig{
					Platform:  "some-platform",
					RootfsURI: "some-image",
					ImageResource: &atc.ImageResource{
						Type:   "docker",
						Source: atc.Source{"some": "super-secret-source"},
						Params: atc.Params{"some": "params"},
					},
					Params: map[string]string{"SOME": "params"},
					Run: atc.TaskRunConfig{
						Path: "ls",
						Args: []string{"some", "args"},
					},
				}

				fakeImageSpec = worker.ImageSpec{
					ImageArtifactSource: new(workerfakes.FakeStreamableArtifactSource),
				}

				fakeDelegate.FetchImageReturns(fakeImageSpec, nil)
			})

			It("succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())
			})

			It("fetches the image", func() {
				Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
				_, imageResource, types, privileged := fakeDelegate.FetchImageArgsForCall(0)
				Expect(imageResource).To(Equal(atc.ImageResource{
					Type:   "docker",
					Source: atc.Source{"some": "super-secret-source"},
					Params: atc.Params{"some": "params"},
				}))
				Expect(types).To(Equal(taskPlan.VersionedResourceTypes))
				Expect(privileged).To(BeFalse())
			})

			It("creates the specs with the image artifact", func() {
				Expect(containerSpec.ImageSpec).To(Equal(fakeImageSpec))
			})

			Context("when tags are specified on the task plan", func() {
				BeforeEach(func() {
					taskPlan.Tags = atc.Tags{"plan", "tags"}
				})

				It("fetches the image with the same tags", func() {
					Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
					_, imageResource, _, _ := fakeDelegate.FetchImageArgsForCall(0)
					Expect(imageResource.Tags).To(Equal(atc.Tags{"plan", "tags"}))
				})
			})

			Context("when tags are specified on the image resource", func() {
				BeforeEach(func() {
					taskPlan.Config.ImageResource.Tags = atc.Tags{"image", "tags"}
				})

				It("fetches the image with the same tags", func() {
					Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
					_, imageResource, _, _ := fakeDelegate.FetchImageArgsForCall(0)
					Expect(imageResource.Tags).To(Equal(atc.Tags{"image", "tags"}))
				})

				Context("when tags are ALSO specified on the task plan", func() {
					BeforeEach(func() {
						taskPlan.Tags = atc.Tags{"plan", "tags"}
					})

					It("fetches the image using only the image tags", func() {
						Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
						_, imageResource, _, _ := fakeDelegate.FetchImageArgsForCall(0)
						Expect(imageResource.Tags).To(Equal(atc.Tags{"image", "tags"}))
					})
				})
			})

			Context("when privileged", func() {
				BeforeEach(func() {
					taskPlan.Privileged = true
				})

				It("fetches a privileged image", func() {
					Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
					_, _, _, privileged := fakeDelegate.FetchImageArgsForCall(0)
					Expect(privileged).To(BeTrue())
				})
			})
		})

		Context("when a run dir is specified", func() {
			var dir string
			BeforeEach(func() {
				dir = "/some/dir"
				taskPlan.Config.Run.Dir = dir
			})

			It("specifies it in the process  spec", func() {
				Expect(processSpec.Dir).To(Equal(dir))
			})
		})

		Context("when a run user is specified", func() {
			BeforeEach(func() {
				taskPlan.Config.Run.User = "some-user"
			})

			It("adds the user to the container spec", func() {
				Expect(containerSpec.User).To(Equal("some-user"))
			})

			It("doesn't bother adding the user to the run spec", func() {
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
						ExitStatus:   taskStepStatus,
						VolumeMounts: []worker.VolumeMount{},
					}
					fakeClient.RunTaskStepReturns(taskResult, nil)
				})
				It("finishes the task via the delegate", func() {
					Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
					_, status, _, _ := fakeDelegate.FinishedArgsForCall(0)
					Expect(status).To(Equal(exec.ExitStatus(taskStepStatus)))
				})

				It("returns successfully", func() {
					Expect(stepErr).ToNot(HaveOccurred())
				})

				Describe("the registered artifacts", func() {
					var (
						artifact1 runtime.Artifact
						artifact2 runtime.Artifact
						artifact3 runtime.Artifact

						fakeMountPath1 string = "some-artifact-root/some-output-configured-path/"
						fakeMountPath2 string = "some-artifact-root/some-other-output/"
						fakeMountPath3 string = "some-artifact-root/some-output-configured-path-with-trailing-slash/"

						fakeVolume1 *workerfakes.FakeVolume
						fakeVolume2 *workerfakes.FakeVolume
						fakeVolume3 *workerfakes.FakeVolume
					)

					BeforeEach(func() {

						fakeVolume1 = new(workerfakes.FakeVolume)
						fakeVolume1.HandleReturns("some-handle-1")
						fakeVolume2 = new(workerfakes.FakeVolume)
						fakeVolume2.HandleReturns("some-handle-2")
						fakeVolume3 = new(workerfakes.FakeVolume)
						fakeVolume3.HandleReturns("some-handle-3")

						fakeTaskResult := worker.TaskResult{
							ExitStatus: 0,
							VolumeMounts: []worker.VolumeMount{
								{
									Volume:    fakeVolume1,
									MountPath: fakeMountPath1,
								},
								{
									Volume:    fakeVolume2,
									MountPath: fakeMountPath2,
								},
								{
									Volume:    fakeVolume3,
									MountPath: fakeMountPath3,
								},
							},
						}
						fakeClient.RunTaskStepReturns(fakeTaskResult, nil)
					})

					JustBeforeEach(func() {
						Expect(stepErr).ToNot(HaveOccurred())

						var found bool
						artifact1, found = repo.ArtifactFor("some-output")
						Expect(found).To(BeTrue())

						artifact2, found = repo.ArtifactFor("some-other-output")
						Expect(found).To(BeTrue())

						artifact3, found = repo.ArtifactFor("some-trailing-slash-output")
						Expect(found).To(BeTrue())
					})

					It("does not register the task as a artifact", func() {
						artifactMap := repo.AsMap()
						Expect(artifactMap).To(ConsistOf(artifact1, artifact2, artifact3))
					})

					It("passes existing output volumes to the resource", func() {
						Expect(containerSpec.Outputs).To(Equal(worker.OutputPaths{
							"some-output":                "some-artifact-root/some-output-configured-path/",
							"some-other-output":          "some-artifact-root/some-other-output/",
							"some-trailing-slash-output": "some-artifact-root/some-output-configured-path-with-trailing-slash/",
						}))
					})
				})
			})

			Context("when the task exits with nonzero status", func() {
				BeforeEach(func() {
					taskStepStatus = 5
					taskResult := worker.TaskResult{ExitStatus: taskStepStatus, VolumeMounts: []worker.VolumeMount{}}
					fakeClient.RunTaskStepReturns(taskResult, nil)
				})
				It("finishes the task via the delegate", func() {
					Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
					_, status, _, _ := fakeDelegate.FinishedArgsForCall(0)
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
				taskResult := worker.TaskResult{ExitStatus: -1, VolumeMounts: []worker.VolumeMount{}}
				fakeClient.RunTaskStepReturns(taskResult, disaster)
			})

			It("returns the error", func() {
				Expect(stepErr).To(Equal(disaster))
			})

			It("is not successful", func() {
				Expect(stepOk).To(BeFalse())
			})
		})

		Context("when the task step is interrupted", func() {
			BeforeEach(func() {
				fakeClient.RunTaskStepReturns(
					worker.TaskResult{
						ExitStatus:   -1,
						VolumeMounts: []worker.VolumeMount{},
					}, context.Canceled)
				cancel()
			})

			It("returns the context.Canceled error", func() {
				Expect(stepErr).To(Equal(context.Canceled))
			})

			It("is not successful", func() {
				Expect(stepOk).To(BeFalse())
			})

			It("waits for RunTaskStep to return", func() {
				Expect(fakeClient.RunTaskStepCallCount()).To(Equal(1))
			})

			It("doesn't register a artifact", func() {
				artifactMap := repo.AsMap()
				Expect(artifactMap).To(BeEmpty())
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

				runTaskStepError error
				taskResult       worker.TaskResult
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

				taskResult = worker.TaskResult{
					ExitStatus: 0,
					VolumeMounts: []worker.VolumeMount{
						{
							Volume:    fakeVolume1,
							MountPath: fakeMountPath1,
						},
						{
							Volume:    fakeVolume2,
							MountPath: fakeMountPath2,
						},
						{
							Volume:    fakeVolume3,
							MountPath: fakeMountPath3,
						},
					},
				}
			})

			var outputsAreRegistered = func() {
				It("registers the outputs as artifacts", func() {
					artifact1, found := repo.ArtifactFor("some-output")
					Expect(found).To(BeTrue())

					artifact2, found := repo.ArtifactFor("some-other-output")
					Expect(found).To(BeTrue())

					artifact3, found := repo.ArtifactFor("some-trailing-slash-output")
					Expect(found).To(BeTrue())

					artifactMap := repo.AsMap()
					Expect(artifactMap).To(ConsistOf(artifact1, artifact2, artifact3))
				})

			}

			Context("when RunTaskStep succeeds", func() {
				BeforeEach(func() {
					runTaskStepError = nil
					fakeClient.RunTaskStepReturns(taskResult, runTaskStepError)
				})

				outputsAreRegistered()
			})

			Context("when RunTaskStep errors", func() {
				BeforeEach(func() {
					runTaskStepError = errors.New("some unexpected error")
					fakeClient.RunTaskStepReturns(taskResult, runTaskStepError)
				})

				outputsAreRegistered()
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
					ExitStatus: 0,
					VolumeMounts: []worker.VolumeMount{
						{
							Volume:    fakeVolume,
							MountPath: fakeMountPath,
						},
					},
				}
				fakeClient.RunTaskStepReturns(taskResult, nil)
			})

			JustBeforeEach(func() {
				Expect(stepErr).ToNot(HaveOccurred())
			})

			It("registers the outputs as artifacts with specific name", func() {
				artifact, found := repo.ArtifactFor("specific-remapped-output")
				Expect(found).To(BeTrue())

				artifactMap := repo.AsMap()
				Expect(artifactMap).To(ConsistOf(artifact))
			})
		})
	})
})
