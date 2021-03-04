package exec_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/concourse/concourse/tracing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/api/trace/tracetest"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/resource/resourcefakes"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/runtime/runtimefakes"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/concourse/concourse/vars"
)

var _ = Describe("PutStep", func() {
	var (
		ctx    context.Context
		cancel func()

		fakeWorker                *workerfakes.FakeWorker
		fakePool                  *workerfakes.FakePool
		fakeClient                *workerfakes.FakeClient
		fakeArtifactSourcer       *workerfakes.FakeArtifactSourcer
		fakeStrategy              *workerfakes.FakeContainerPlacementStrategy
		fakeResourceFactory       *resourcefakes.FakeResourceFactory
		fakeResource              *resourcefakes.FakeResource
		fakeResourceConfigFactory *dbfakes.FakeResourceConfigFactory
		fakeDelegate              *execfakes.FakePutDelegate
		fakeDelegateFactory       *execfakes.FakePutDelegateFactory

		expectedInputs []worker.InputSource

		spanCtx context.Context

		putPlan *atc.PutPlan

		fakeArtifact        *runtimefakes.FakeArtifact
		fakeOtherArtifact   *runtimefakes.FakeArtifact
		fakeMountedArtifact *runtimefakes.FakeArtifact

		interpolatedResourceTypes atc.VersionedResourceTypes

		containerMetadata = db.ContainerMetadata{
			WorkingDirectory: resource.ResourcesDir("put"),
			Type:             db.ContainerTypePut,
			StepName:         "some-step",
		}

		stepMetadata = exec.StepMetadata{
			TeamID:       123,
			TeamName:     "some-team",
			BuildID:      42,
			BuildName:    "some-build",
			PipelineID:   4567,
			PipelineName: "some-pipeline",
		}

		repo  *build.Repository
		state *execfakes.FakeRunState

		putStep exec.Step
		stepOk  bool
		stepErr error

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		planID atc.PlanID

		versionResult runtime.VersionResult

		shouldRunPutStep bool
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		planID = atc.PlanID("some-plan-id")

		fakeClient = new(workerfakes.FakeClient)
		fakeClient.NameReturns("some-worker")
		fakePool = new(workerfakes.FakePool)
		fakePool.WaitForWorkerReturns(fakeClient, 0, nil)

		fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)
		fakeArtifactSourcer = new(workerfakes.FakeArtifactSourcer)
		fakeWorker = new(workerfakes.FakeWorker)
		fakeResourceFactory = new(resourcefakes.FakeResourceFactory)
		fakeResourceConfigFactory = new(dbfakes.FakeResourceConfigFactory)

		expectedInputs = []worker.InputSource{new(workerfakes.FakeInputSource)}
		fakeArtifactSourcer.SourceInputsAndCachesReturns(expectedInputs, nil)

		fakeDelegate = new(execfakes.FakePutDelegate)
		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
		fakeDelegate.StdoutReturns(stdoutBuf)
		fakeDelegate.StderrReturns(stderrBuf)

		fakeDelegateFactory = new(execfakes.FakePutDelegateFactory)
		fakeDelegateFactory.PutDelegateReturns(fakeDelegate)

		spanCtx = context.Background()
		fakeDelegate.StartSpanReturns(spanCtx, trace.NoopSpan{})

		versionResult = runtime.VersionResult{
			Version:  atc.Version{"some": "version"},
			Metadata: []atc.MetadataField{{Name: "some", Value: "metadata"}},
		}

		fakeResource = new(resourcefakes.FakeResource)
		fakeResource.PutReturns(versionResult, nil)

		repo = build.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactRepositoryReturns(repo)

		state.GetStub = vars.StaticVariables{
			"source-var": "super-secret-source",
			"params-var": "super-secret-params",
		}.Get

		uninterpolatedResourceTypes := atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "some-custom-type",
					Type:   "another-custom-type",
					Source: atc.Source{"some-custom": "((source-var))"},
					Params: atc.Params{"some-custom": "((params-var))"},
				},
				Version: atc.Version{"some-custom": "version"},
			},
			{
				ResourceType: atc.ResourceType{
					Name:       "another-custom-type",
					Type:       "registry-image",
					Source:     atc.Source{"another-custom": "((source-var))"},
					Privileged: true,
				},
				Version: atc.Version{"another-custom": "version"},
			},
		}

		interpolatedResourceTypes = atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "some-custom-type",
					Type:   "another-custom-type",
					Source: atc.Source{"some-custom": "super-secret-source"},

					// params don't need to be interpolated because it's used for
					// fetching, not constructing the resource config
					Params: atc.Params{"some-custom": "((params-var))"},
				},
				Version: atc.Version{"some-custom": "version"},
			},
			{
				ResourceType: atc.ResourceType{
					Name:       "another-custom-type",
					Type:       "registry-image",
					Source:     atc.Source{"another-custom": "super-secret-source"},
					Privileged: true,
				},
				Version: atc.Version{"another-custom": "version"},
			},
		}

		putPlan = &atc.PutPlan{
			Name:                   "some-name",
			Resource:               "some-resource",
			Type:                   "some-resource-type",
			Source:                 atc.Source{"some": "((source-var))"},
			Params:                 atc.Params{"some": "((params-var))"},
			VersionedResourceTypes: uninterpolatedResourceTypes,
		}

		fakeArtifact = new(runtimefakes.FakeArtifact)
		fakeOtherArtifact = new(runtimefakes.FakeArtifact)
		fakeMountedArtifact = new(runtimefakes.FakeArtifact)

		repo.RegisterArtifact("some-source", fakeArtifact)
		repo.RegisterArtifact("some-other-source", fakeOtherArtifact)
		repo.RegisterArtifact("some-mounted-source", fakeMountedArtifact)

		fakeResourceFactory.NewResourceReturns(fakeResource)

		fakeClient.RunPutStepReturns(
			worker.PutResult{ExitStatus: 0, VersionResult: versionResult},
			nil,
		)

		shouldRunPutStep = true
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
			fakeResourceFactory,
			fakeResourceConfigFactory,
			fakeStrategy,
			fakePool,
			fakeArtifactSourcer,
			fakeDelegateFactory,
		)

		stepOk, stepErr = putStep.Run(ctx, state)
	})

	var runCtx context.Context
	var owner db.ContainerOwner
	var containerSpec worker.ContainerSpec
	var metadata db.ContainerMetadata
	var processSpec runtime.ProcessSpec
	var startEventDelegate runtime.StartingEventDelegate
	var runResource resource.Resource

	JustBeforeEach(func() {
		if shouldRunPutStep {
			Expect(fakeClient.RunPutStepCallCount()).To(Equal(1), "put step should have run")
			runCtx, owner, containerSpec, metadata, processSpec, startEventDelegate, runResource = fakeClient.RunPutStepArgsForCall(0)
		} else {
			Expect(fakeClient.RunPutStepCallCount()).To(Equal(0), "put step should NOT have run")
		}
	})

	Describe("worker selection", func() {
		var workerSpec worker.WorkerSpec

		JustBeforeEach(func() {
			Expect(fakePool.WaitForWorkerCallCount()).To(Equal(1))
			_, _, _, workerSpec, _, _ = fakePool.WaitForWorkerArgsForCall(0)
		})

		It("calls WaitForWorker with the correct WorkerSpec", func() {
			Expect(workerSpec).To(Equal(
				worker.WorkerSpec{
					ResourceType: "some-resource-type",
					TeamID:       stepMetadata.TeamID,
				},
			))
		})

		It("emits a SelectedWorker event", func() {
			Expect(fakeDelegate.SelectedWorkerCallCount()).To(Equal(1))
			_, workerName := fakeDelegate.SelectedWorkerArgsForCall(0)
			Expect(workerName).To(Equal("some-worker"))
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
				fakePool.WaitForWorkerReturns(nil, 0, errors.New("nope"))
				shouldRunPutStep = false
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

			It("calls RunPutStep with all inputs", func() {
				Expect(fakeArtifactSourcer.SourceInputsAndCachesCallCount()).To(Equal(1))
				_, teamID, inputMap := fakeArtifactSourcer.SourceInputsAndCachesArgsForCall(0)
				Expect(teamID).To(Equal(123))
				Expect(inputMap).To(HaveLen(3))
				Expect(inputMap["/tmp/build/put/some-other-source"]).To(Equal(fakeOtherArtifact))
				Expect(inputMap["/tmp/build/put/some-mounted-source"]).To(Equal(fakeMountedArtifact))
				Expect(inputMap["/tmp/build/put/some-source"]).To(Equal(fakeArtifact))
			})
		})

		Context("when inputs are left blank", func() {
			It("calls RunPutStep with all inputs", func() {
				_, _, inputMap := fakeArtifactSourcer.SourceInputsAndCachesArgsForCall(0)
				Expect(inputMap).To(HaveLen(3))
				Expect(inputMap["/tmp/build/put/some-other-source"]).To(Equal(fakeOtherArtifact))
				Expect(inputMap["/tmp/build/put/some-mounted-source"]).To(Equal(fakeMountedArtifact))
				Expect(inputMap["/tmp/build/put/some-source"]).To(Equal(fakeArtifact))
			})
		})

		Context("when only some inputs are specified ", func() {
			BeforeEach(func() {
				putPlan.Inputs = &atc.InputsConfig{
					Specified: []string{"some-source", "some-other-source"},
				}
			})

			It("calls RunPutStep with specified inputs", func() {
				_, _, inputMap := fakeArtifactSourcer.SourceInputsAndCachesArgsForCall(0)
				Expect(inputMap).To(HaveLen(2))
				Expect(inputMap["/tmp/build/put/some-other-source"]).To(Equal(fakeOtherArtifact))
				Expect(inputMap["/tmp/build/put/some-source"]).To(Equal(fakeArtifact))
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
						"some-param":    "some-source/source",
						"another-param": "does-not-exist",
						"number-param":  123,
					}
				})

				It("calls RunPutStep with detected inputs", func() {
					_, _, inputMap := fakeArtifactSourcer.SourceInputsAndCachesArgsForCall(0)
					Expect(inputMap).To(HaveLen(1))
					Expect(inputMap["/tmp/build/put/some-source"]).To(Equal(fakeArtifact))
				})
			})

			Context("when the params have maps and slices", func() {
				BeforeEach(func() {
					putPlan.Params = atc.Params{
						"some-slice": []interface{}{
							[]interface{}{"some-source/source", "does-not-exist", 123},
							[]interface{}{"does not exist-2"},
						},
						"some-map": map[string]interface{}{
							"key": "some-other-source/source",
						},
					}
				})

				It("calls RunPutStep with detected inputs", func() {
					_, _, inputMap := fakeArtifactSourcer.SourceInputsAndCachesArgsForCall(0)
					Expect(inputMap).To(HaveLen(2))
					Expect(inputMap["/tmp/build/put/some-other-source"]).To(Equal(fakeOtherArtifact))
					Expect(inputMap["/tmp/build/put/some-source"]).To(Equal(fakeArtifact))
				})
			})
		})
	})

	It("calls workerClient -> RunPutStep with the appropriate arguments", func() {
		Expect(runCtx).To(Equal(rewrapLogger(spanCtx)))
		Expect(owner).To(Equal(db.NewBuildStepContainerOwner(42, atc.PlanID(planID), 123)))
		Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
			ResourceType: "some-resource-type",
		}))
		Expect(containerSpec.TeamID).To(Equal(123))
		Expect(containerSpec.Env).To(Equal(stepMetadata.Env()))
		Expect(containerSpec.Dir).To(Equal("/tmp/build/put"))
		Expect(containerSpec.Inputs).To(Equal(expectedInputs))

		Expect(metadata).To(Equal(containerMetadata))

		Expect(processSpec).To(Equal(
			runtime.ProcessSpec{
				Path:         "/opt/resource/out",
				Args:         []string{resource.ResourcesDir("put")},
				StdoutWriter: stdoutBuf,
				StderrWriter: stderrBuf,
			}))
		Expect(startEventDelegate).To(Equal(fakeDelegate))
		Expect(runResource).To(Equal(fakeResource))
	})

	Context("when using a custom resource type", func() {
		var fakeImageSpec worker.ImageSpec

		BeforeEach(func() {
			putPlan.Type = "some-custom-type"

			fakeImageSpec = worker.ImageSpec{
				ImageArtifactSource: new(workerfakes.FakeStreamableArtifactSource),
			}

			fakeDelegate.FetchImageReturns(fakeImageSpec, nil)
		})

		It("fetches the resource type image and uses it for the container", func() {
			Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))

			_, imageResource, types, privileged := fakeDelegate.FetchImageArgsForCall(0)

			By("fetching the type image")
			Expect(imageResource).To(Equal(atc.ImageResource{
				Name:    "some-custom-type",
				Type:    "another-custom-type",
				Source:  atc.Source{"some-custom": "((source-var))"},
				Params:  atc.Params{"some-custom": "((params-var))"},
				Version: atc.Version{"some-custom": "version"},
			}))

			By("excluding the type from the FetchImage call")
			Expect(types).To(Equal(atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Name:       "another-custom-type",
						Type:       "registry-image",
						Source:     atc.Source{"another-custom": "((source-var))"},
						Privileged: true,
					},
					Version: atc.Version{"another-custom": "version"},
				},
			}))

			By("not being privileged")
			Expect(privileged).To(BeFalse())
		})

		It("sets the bottom-most type in the worker spec", func() {
			Expect(fakePool.WaitForWorkerCallCount()).To(Equal(1))
			_, _, _, workerSpec, _, _ := fakePool.WaitForWorkerArgsForCall(0)

			Expect(workerSpec).To(Equal(worker.WorkerSpec{
				TeamID:       stepMetadata.TeamID,
				ResourceType: "registry-image",
			}))
		})

		It("sets the image spec in the container spec", func() {
			Expect(containerSpec.ImageSpec).To(Equal(fakeImageSpec))
		})

		Context("when the resource type is privileged", func() {
			BeforeEach(func() {
				putPlan.Type = "another-custom-type"
			})

			It("fetches the image with privileged", func() {
				Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
				_, _, _, privileged := fakeDelegate.FetchImageArgsForCall(0)
				Expect(privileged).To(BeTrue())
			})
		})

		Context("when the plan configures tags", func() {
			BeforeEach(func() {
				putPlan.Tags = atc.Tags{"plan", "tags"}
			})

			It("fetches using the tags", func() {
				Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
				_, imageResource, _, _ := fakeDelegate.FetchImageArgsForCall(0)
				Expect(imageResource.Tags).To(Equal(atc.Tags{"plan", "tags"}))
			})
		})

		Context("when the resource type configures tags", func() {
			BeforeEach(func() {
				taggedType, found := putPlan.VersionedResourceTypes.Lookup("some-custom-type")
				Expect(found).To(BeTrue())

				taggedType.Tags = atc.Tags{"type", "tags"}

				newTypes := putPlan.VersionedResourceTypes.Without("some-custom-type")
				newTypes = append(newTypes, taggedType)

				putPlan.VersionedResourceTypes = newTypes
			})

			It("fetches using the type tags", func() {
				Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
				_, imageResource, _, _ := fakeDelegate.FetchImageArgsForCall(0)
				Expect(imageResource.Tags).To(Equal(atc.Tags{"type", "tags"}))
			})

			Context("when the plan ALSO configures tags", func() {
				BeforeEach(func() {
					putPlan.Tags = atc.Tags{"plan", "tags"}
				})

				It("fetches using only the type tags", func() {
					Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
					_, imageResource, _, _ := fakeDelegate.FetchImageArgsForCall(0)
					Expect(imageResource.Tags).To(Equal(atc.Tags{"type", "tags"}))
				})
			})
		})
	})

	Context("when the plan specifies a timeout", func() {
		BeforeEach(func() {
			putPlan.Timeout = "1h"
		})

		It("enforces it on the put", func() {
			t, ok := runCtx.Deadline()
			Expect(ok).To(BeTrue())
			Expect(t).To(BeTemporally("~", time.Now().Add(time.Hour), time.Minute))
		})

		Context("when running times out", func() {
			BeforeEach(func() {
				fakeClient.RunPutStepReturns(
					worker.PutResult{},
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
				putPlan.Timeout = "bogus"
				shouldRunPutStep = false
			})

			It("fails miserably", func() {
				Expect(stepErr).To(MatchError("parse timeout: time: invalid duration \"bogus\""))
			})
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

	Context("when creds tracker can initialize the resource", func() {
		var (
			fakeResourceConfig *dbfakes.FakeResourceConfig
		)

		BeforeEach(func() {
			fakeResourceConfig = new(dbfakes.FakeResourceConfig)
			fakeResourceConfig.IDReturns(1)

			fakeResourceConfigFactory.FindOrCreateResourceConfigReturns(fakeResourceConfig, nil)

			fakeWorker.NameReturns("some-worker")
		})

		It("creates a resource with the correct source and params", func() {
			actualSource, actualParams, _ := fakeResourceFactory.NewResourceArgsForCall(0)
			Expect(actualSource).To(Equal(atc.Source{"some": "super-secret-source"}))
			Expect(actualParams).To(Equal(atc.Params{"some": "super-secret-params"}))

			Expect(runResource).To(Equal(fakeResource))
		})

	})

	It("saves the build output", func() {
		Expect(fakeDelegate.SaveOutputCallCount()).To(Equal(1))

		_, plan, actualSource, actualResourceTypes, info := fakeDelegate.SaveOutputArgsForCall(0)
		Expect(plan.Name).To(Equal("some-name"))
		Expect(plan.Type).To(Equal("some-resource-type"))
		Expect(plan.Resource).To(Equal("some-resource"))
		Expect(actualSource).To(Equal(atc.Source{"some": "super-secret-source"}))
		Expect(actualResourceTypes).To(Equal(interpolatedResourceTypes))
		Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
		Expect(info.Metadata).To(Equal([]atc.MetadataField{{Name: "some", Value: "metadata"}}))
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

	Context("when RunPutStep succeeds", func() {
		It("finishes via the delegate", func() {
			Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
			_, status, info := fakeDelegate.FinishedArgsForCall(0)
			Expect(status).To(Equal(exec.ExitStatus(0)))
			Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
			Expect(info.Metadata).To(Equal([]atc.MetadataField{{Name: "some", Value: "metadata"}}))
		})

		It("stores the version result as the step result", func() {
			Expect(state.StoreResultCallCount()).To(Equal(1))
			sID, sVal := state.StoreResultArgsForCall(0)
			Expect(sID).To(Equal(planID))
			Expect(sVal).To(Equal(versionResult))
		})

		It("is successful", func() {
			Expect(stepOk).To(BeTrue())
		})
	})

	Context("when RunPutStep exits unsuccessfully", func() {
		BeforeEach(func() {
			versionResult = runtime.VersionResult{}

			fakeClient.RunPutStepReturns(
				worker.PutResult{ExitStatus: 42, VersionResult: versionResult},
				nil,
			)
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

	Context("when RunPutStep exits with an error", func() {
		disaster := errors.New("oh no")

		BeforeEach(func() {
			fakeClient.RunPutStepReturns(worker.PutResult{}, disaster)
		})

		It("does not finish the step via the delegate", func() {
			Expect(fakeDelegate.FinishedCallCount()).To(Equal(0))
		})

		It("returns the error", func() {
			Expect(stepErr).To(Equal(disaster))
		})

		It("is not successful", func() {
			Expect(stepOk).To(BeFalse())
		})
	})
})
