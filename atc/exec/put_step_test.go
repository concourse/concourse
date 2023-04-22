package exec_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/concourse/concourse/tracing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"go.opentelemetry.io/otel/oteltest"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/runtime/runtimetest"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/vars"
)

var _ = Describe("PutStep", func() {
	var (
		ctx    context.Context
		cancel func()

		fakeDelegate        *execfakes.FakePutDelegate
		fakeDelegateFactory *execfakes.FakePutDelegateFactory

		fakePool        *execfakes.FakePool
		chosenWorker    *runtimetest.Worker
		chosenContainer *runtimetest.WorkerContainer

		spanCtx context.Context

		putPlan *atc.PutPlan

		volume1 *runtimetest.Volume
		volume2 *runtimetest.Volume
		volume3 *runtimetest.Volume

		containerMetadata = db.ContainerMetadata{
			WorkingDirectory: resource.ResourcesDir("put"),
			Type:             db.ContainerTypePut,
			StepName:         "some-step",
		}

		planID       = atc.PlanID("some-plan-id")
		stepMetadata = exec.StepMetadata{
			TeamID:       123,
			TeamName:     "some-team",
			BuildID:      42,
			BuildName:    "some-build",
			PipelineID:   4567,
			PipelineName: "some-pipeline",
		}
		expectedOwner = db.NewBuildStepContainerOwner(stepMetadata.BuildID, planID, stepMetadata.TeamID)

		state exec.RunState
		repo  *build.Repository

		putStep exec.Step
		stepOk  bool
		stepErr error

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		versionResult resource.VersionResult

		defaultPutTimeout time.Duration = 0
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		versionResult = resource.VersionResult{
			Version:  atc.Version{"some": "version"},
			Metadata: []atc.MetadataField{{Name: "some", Value: "metadata"}},
		}

		chosenWorker = runtimetest.NewWorker("worker").
			WithContainer(
				expectedOwner,
				runtimetest.NewContainer().WithProcess(
					runtime.ProcessSpec{
						ID:   "resource",
						Path: "/opt/resource/out",
						Args: []string{resource.ResourcesDir("put")},
					},
					runtimetest.ProcessStub{
						Attachable: true,
						Output:     versionResult,
					},
				),
				nil,
			)
		chosenContainer = chosenWorker.Containers[0]
		fakePool = new(execfakes.FakePool)
		fakePool.FindOrSelectWorkerReturns(chosenWorker, nil)

		fakeDelegate = new(execfakes.FakePutDelegate)
		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
		fakeDelegate.StdoutReturns(stdoutBuf)
		fakeDelegate.StderrReturns(stderrBuf)

		fakeDelegateFactory = new(execfakes.FakePutDelegateFactory)
		fakeDelegateFactory.PutDelegateReturns(fakeDelegate)

		spanCtx = context.Background()
		fakeDelegate.StartSpanReturns(spanCtx, tracing.NoopSpan)

		state = exec.NewRunState(noopStepper, vars.StaticVariables{
			"source-var": "super-secret-source",
			"params-var": "super-secret-params",
		}, false)
		repo = state.ArtifactRepository()

		putPlan = &atc.PutPlan{
			Name:     "some-name",
			Resource: "some-resource",
			Type:     "some-resource-type",
			TypeImage: atc.TypeImage{
				BaseType: "some-resource-type",
			},
			Source: atc.Source{"some": "((source-var))"},
			Params: atc.Params{"some": "((params-var))"},
		}

		volume1 = runtimetest.NewVolume("volume1")
		volume2 = runtimetest.NewVolume("volume2")
		volume3 = runtimetest.NewVolume("volume3")

		repo.RegisterArtifact("input1", volume1, false)
		repo.RegisterArtifact("input2", volume2, true)
		repo.RegisterArtifact("input3", volume3, false)
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		plan := atc.Plan{
			ID:  atc.PlanID(planID),
			Put: putPlan,
		}

		putStep = exec.NewPutStep(
			plan.ID,
			*plan.Put,
			stepMetadata,
			containerMetadata,
			nil,
			fakePool,
			fakeDelegateFactory,
			defaultPutTimeout,
		)

		stepOk, stepErr = putStep.Run(ctx, state)
		if stepErr != nil {
			testLogger.Error("putStep.Run-failed", stepErr)
		}
	})

	Describe("worker selection", func() {
		var ctx context.Context
		var workerSpec worker.Spec

		JustBeforeEach(func() {
			Expect(fakePool.FindOrSelectWorkerCallCount()).To(Equal(1))
			ctx, _, _, workerSpec, _, _ = fakePool.FindOrSelectWorkerArgsForCall(0)
		})

		It("doesn't enforce a timeout", func() {
			_, ok := ctx.Deadline()
			Expect(ok).To(BeFalse())
		})

		It("emits a BeforeSelectWorker event", func() {
			Expect(fakeDelegate.BeforeSelectWorkerCallCount()).To(Equal(1))
		})

		It("calls SelectWorker with the correct WorkerSpec", func() {
			Expect(workerSpec).To(Equal(
				worker.Spec{
					ResourceType: "some-resource-type",
					TeamID:       stepMetadata.TeamID,
				},
			))
		})

		It("emits a SelectedWorker event", func() {
			Expect(fakeDelegate.SelectedWorkerCallCount()).To(Equal(1))
			_, workerName := fakeDelegate.SelectedWorkerArgsForCall(0)
			Expect(workerName).To(Equal("worker"))
		})

		Context("when the plan specifies tags", func() {
			BeforeEach(func() {
				putPlan.Tags = atc.Tags{"some", "tags"}
			})

			It("sets them in the WorkerSpec", func() {
				Expect(workerSpec.Tags).To(Equal([]string{"some", "tags"}))
			})
		})

		Context("when selecting a worker fails", func() {
			BeforeEach(func() {
				fakePool.FindOrSelectWorkerReturns(nil, errors.New("nope"))
			})

			It("returns an err", func() {
				Expect(stepErr).To(MatchError(ContainSubstring("nope")))
			})
		})
	})

	Context("inputs", func() {
		Context("when inputs are specified with 'all' keyword", func() {
			BeforeEach(func() {
				putPlan.Inputs = &atc.InputsConfig{
					All: true,
				}
			})

			It("runs with all inputs", func() {
				Expect(chosenContainer.Spec.Inputs).To(ConsistOf([]runtime.Input{
					{
						Artifact:        volume1,
						DestinationPath: "/tmp/build/put/input1",
						FromCache:       false,
					},
					{
						Artifact:        volume2,
						DestinationPath: "/tmp/build/put/input2",
						FromCache:       true,
					},
					{
						Artifact:        volume3,
						DestinationPath: "/tmp/build/put/input3",
						FromCache:       false,
					},
				}))
			})
		})

		Context("when inputs are left blank", func() {
			It("runs with all inputs", func() {
				Expect(chosenContainer.Spec.Inputs).To(ConsistOf([]runtime.Input{
					{
						Artifact:        volume1,
						DestinationPath: "/tmp/build/put/input1",
						FromCache:       false,
					},
					{
						Artifact:        volume2,
						DestinationPath: "/tmp/build/put/input2",
						FromCache:       true,
					},
					{
						Artifact:        volume3,
						DestinationPath: "/tmp/build/put/input3",
						FromCache:       false,
					},
				}))
			})
		})

		Context("when only some inputs are specified ", func() {
			BeforeEach(func() {
				putPlan.Inputs = &atc.InputsConfig{
					Specified: []string{"input1", "input3"},
				}
			})

			It("runs with specified inputs", func() {
				Expect(chosenContainer.Spec.Inputs).To(ConsistOf([]runtime.Input{
					{
						Artifact:        volume1,
						DestinationPath: "/tmp/build/put/input1",
						FromCache:       false,
					},
					{
						Artifact:        volume3,
						DestinationPath: "/tmp/build/put/input3",
						FromCache:       false,
					},
				}))
			})
		})

		Context("when an empty list of inputs is specified", func() {
			BeforeEach(func() {
				putPlan.Inputs = &atc.InputsConfig{
					Specified: []string{},
				}
			})

			It("runs with no inputs", func() {
				Expect(chosenContainer.Spec.Inputs).To(Equal([]runtime.Input{}))
			})
		})

		Context("when the inputs are detected", func() {
			BeforeEach(func() {
				putPlan.Inputs = &atc.InputsConfig{
					Detect: true,
				}
			})

			Context("when the params are only strings", func() {
				BeforeEach(func() {
					putPlan.Params = atc.Params{
						"some-param":    "input1/source",
						"another-param": "does-not-exist",
						"number-param":  123,
					}
				})

				It("runs with detected inputs", func() {
					Expect(chosenContainer.Spec.Inputs).To(ConsistOf([]runtime.Input{
						{
							Artifact:        volume1,
							DestinationPath: "/tmp/build/put/input1",
						},
					}))
				})
			})

			Context("when the params have maps and slices", func() {
				BeforeEach(func() {
					putPlan.Params = atc.Params{
						"some-slice": []interface{}{
							[]interface{}{"input1/source", "does-not-exist", 123},
							[]interface{}{"does not exist-2"},
						},
						"some-map": map[string]interface{}{
							"key": "input2/source",
						},
					}
				})

				It("runs with detected inputs", func() {
					Expect(chosenContainer.Spec.Inputs).To(ConsistOf([]runtime.Input{
						{
							Artifact:        volume1,
							DestinationPath: "/tmp/build/put/input1",
							FromCache:       false,
						},
						{
							Artifact:        volume2,
							DestinationPath: "/tmp/build/put/input2",
							FromCache:       true,
						},
					}))
				})
			})

			Context("when the params contains . and ..", func() {
				BeforeEach(func() {
					putPlan.Params = atc.Params{
						"some-param": "./input1/source",
						"some-map": map[string]interface{}{
							"key": "../input2/source",
						},
					}
				})

				It("runs with detected inputs", func() {
					Expect(chosenContainer.Spec.Inputs).To(ConsistOf([]runtime.Input{
						{
							Artifact:        volume1,
							DestinationPath: "/tmp/build/put/input1",
							FromCache:       false,
						},
						{
							Artifact:        volume2,
							DestinationPath: "/tmp/build/put/input2",
							FromCache:       true,
						},
					}))
				})
			})

			Context("when only inputs are from cache ", func() {
				BeforeEach(func() {
					putPlan.Inputs = &atc.InputsConfig{
						Specified: []string{"input2"},
					}
				})

				It("runs with cached inputs", func() {
					Expect(chosenContainer.Spec.Inputs).To(ConsistOf([]runtime.Input{
						{
							Artifact:        volume2,
							DestinationPath: "/tmp/build/put/input2",
							FromCache:       true,
						},
					}))
				})
			})
		})
	})

	It("saves the build output", func() {
		Expect(fakeDelegate.SaveOutputCallCount()).To(Equal(1))

		_, plan, actualSource, irc, info := fakeDelegate.SaveOutputArgsForCall(0)
		Expect(plan.Name).To(Equal("some-name"))
		Expect(plan.Type).To(Equal("some-resource-type"))
		Expect(plan.Resource).To(Equal("some-resource"))
		Expect(actualSource).To(Equal(atc.Source{"some": "super-secret-source"}))
		Expect(irc).To(BeNil())
		Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
		Expect(info.Metadata).To(Equal([]atc.MetadataField{{Name: "some", Value: "metadata"}}))
	})

	Context("when using a custom resource type", func() {
		var (
			fetchedImageSpec       runtime.ImageSpec
			fakeImageResourceCache *dbfakes.FakeResourceCache
		)

		BeforeEach(func() {
			putPlan.TypeImage.GetPlan = &atc.Plan{
				ID: "1/image-get",
				Get: &atc.GetPlan{
					Name:   "some-custom-type",
					Type:   "another-custom-type",
					Source: atc.Source{"some-custom": "((source-var))"},
					Params: atc.Params{"some-custom": "((params-var))"},
				},
			}

			putPlan.TypeImage.CheckPlan = &atc.Plan{
				ID: "1/image-check",
				Check: &atc.CheckPlan{
					Name:   "some-custom-type",
					Type:   "another-custom-type",
					Source: atc.Source{"some-custom": "((source-var))"},
				},
			}

			putPlan.Type = "some-custom-type"
			putPlan.TypeImage.BaseType = "registry-image"

			fetchedImageSpec = runtime.ImageSpec{
				ImageArtifact: runtimetest.NewVolume("some-volume"),
			}

			fakeDelegate.FetchImageReturns(fetchedImageSpec, fakeImageResourceCache, nil)
		})

		It("fetches the resource type image and uses it for the container", func() {
			Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
			_, actualGetImagePlan, actualCheckImagePlan, privileged := fakeDelegate.FetchImageArgsForCall(0)
			Expect(actualGetImagePlan).To(Equal(*putPlan.TypeImage.GetPlan))
			Expect(actualCheckImagePlan).To(Equal(putPlan.TypeImage.CheckPlan))
			Expect(privileged).To(BeFalse())
		})

		It("sets the bottom-most type in the worker spec", func() {
			Expect(fakePool.FindOrSelectWorkerCallCount()).To(Equal(1))
			_, _, _, workerSpec, _, _ := fakePool.FindOrSelectWorkerArgsForCall(0)

			Expect(workerSpec).To(Equal(worker.Spec{
				TeamID:       stepMetadata.TeamID,
				ResourceType: "registry-image",
			}))
		})

		It("sets the image spec in the container spec", func() {
			Expect(chosenContainer.Spec.ImageSpec).To(Equal(fetchedImageSpec))
		})

		It("saves the build output using the custom type's resource cache", func() {
			Expect(fakeDelegate.SaveOutputCallCount()).To(Equal(1))

			_, _, _, irc, _ := fakeDelegate.SaveOutputArgsForCall(0)
			Expect(irc).To(Equal(fakeImageResourceCache))
		})

		Context("when the resource type is privileged", func() {
			BeforeEach(func() {
				putPlan.TypeImage.Privileged = true
			})

			It("fetches the image with privileged", func() {
				Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
				_, _, _, privileged := fakeDelegate.FetchImageArgsForCall(0)
				Expect(privileged).To(BeTrue())
			})
		})
	})

	Context("when the plan specifies a timeout", func() {
		BeforeEach(func() {
			putPlan.Timeout = "1ms"

			chosenContainer.ProcessDefs[0].Stub.Do = func(ctx context.Context, _ *runtimetest.Process) error {
				select {
				case <-ctx.Done():
					return fmt.Errorf("wrapped: %w", ctx.Err())
				case <-time.After(100 * time.Millisecond):
					return nil
				}
			}
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

		Context("when the timeout is bogus", func() {
			BeforeEach(func() {
				putPlan.Timeout = "bogus"
			})

			It("fails miserably", func() {
				Expect(stepErr).To(MatchError("parse timeout: time: invalid duration \"bogus\""))
			})
		})
	})

	Context("when there is default put timeout", func() {
		BeforeEach(func() {
			defaultPutTimeout = time.Minute * 30
		})

		It("enforces it on the put", func() {
			t, ok := chosenContainer.ContextOfRun().Deadline()
			Expect(ok).To(BeTrue())
			Expect(t).To(BeTemporally("~", time.Now().Add(time.Minute*30), time.Minute))
		})
	})

	Context("when there is default put timeout and the plan specifies a timeout also", func() {
		BeforeEach(func() {
			defaultPutTimeout = time.Minute * 30
			putPlan.Timeout = "1h"
		})

		It("enforces the plan's timeout on the put", func() {
			t, ok := chosenContainer.ContextOfRun().Deadline()
			Expect(ok).To(BeTrue())
			Expect(t).To(BeTemporally("~", time.Now().Add(time.Hour), time.Minute))
		})
	})

	Context("when tracing is enabled", func() {
		BeforeEach(func() {
			tracing.ConfigureTraceProvider(oteltest.NewTracerProvider())

			spanCtx, buildSpan := tracing.StartSpan(ctx, "build", nil)
			fakeDelegate.StartSpanReturns(spanCtx, buildSpan)

			chosenContainer.ProcessDefs[0].Stub.Do = func(ctx context.Context, _ *runtimetest.Process) error {
				defer GinkgoRecover()
				// Properly propagates span context
				Expect(tracing.FromContext(ctx)).To(Equal(buildSpan))
				return nil
			}
		})

		AfterEach(func() {
			tracing.Configured = false
		})

		It("populates the TRACEPARENT env var", func() {
			Expect(chosenContainer.Spec.Env).To(ContainElement(MatchRegexp(`TRACEPARENT=.+`)))
		})
	})

	Describe("invoked resource", func() {
		var invokedResource resource.Resource

		BeforeEach(func() {
			chosenContainer.ProcessDefs[0].Stub.Do = func(_ context.Context, p *runtimetest.Process) error {
				return json.NewDecoder(p.Stdin()).Decode(&invokedResource)
			}
		})

		It("runs the script with the correct source and params", func() {
			Expect(invokedResource.Source).To(Equal(atc.Source{"some": "super-secret-source"}))
			Expect(invokedResource.Params).To(Equal(atc.Params{"some": "super-secret-params"}))
		})
	})

	Context("when the step.Plan.Resource is blank", func() {
		BeforeEach(func() {
			putPlan.Resource = ""
		})

		It("is successful", func() {
			Expect(stepOk).To(BeTrue())
		})

		It("does not save the build output", func() {
			Expect(fakeDelegate.SaveOutputCallCount()).To(Equal(0))
		})
	})

	Context("when the script succeeds", func() {
		It("finishes via the delegate", func() {
			Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
			_, status, info := fakeDelegate.FinishedArgsForCall(0)
			Expect(status).To(Equal(exec.ExitStatus(0)))
			Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
			Expect(info.Metadata).To(Equal([]atc.MetadataField{{Name: "some", Value: "metadata"}}))
		})

		It("stores the version as the step result", func() {
			var val atc.Version
			Expect(state.Result(planID, &val)).To(BeTrue())
			Expect(val).To(Equal(versionResult.Version))
		})

		It("is successful", func() {
			Expect(stepOk).To(BeTrue())
		})
	})

	Context("when running the script exits unsuccessfully", func() {
		BeforeEach(func() {
			chosenContainer.ProcessDefs[0].Stub.ExitStatus = 42
		})

		It("finishes the step via the delegate", func() {
			Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
			_, status, info := fakeDelegate.FinishedArgsForCall(0)
			Expect(status).To(Equal(exec.ExitStatus(42)))
			Expect(info).To(BeZero())
		})

		It("returns nil", func() {
			Expect(stepErr).ToNot(HaveOccurred())
		})

		It("is not successful", func() {
			Expect(stepOk).To(BeFalse())
		})
	})

	Context("when running the script exits with an error", func() {
		disaster := errors.New("oh no")

		BeforeEach(func() {
			chosenContainer.ProcessDefs[0].Stub.Err = disaster.Error()
		})

		It("does not finish the step via the delegate", func() {
			Expect(fakeDelegate.FinishedCallCount()).To(Equal(0))
		})

		It("returns the error", func() {
			Expect(stepErr).To(MatchError(disaster))
		})

		It("is not successful", func() {
			Expect(stepOk).To(BeFalse())
		})
	})
})
