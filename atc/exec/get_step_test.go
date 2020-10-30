package exec_test

import (
	"context"
	"errors"

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
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/vars"
	"github.com/onsi/gomega/gbytes"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/api/trace/tracetest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetStep", func() {
	var (
		ctx       context.Context
		cancel    func()
		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		fakeClient   *workerfakes.FakeClient
		fakeWorker   *workerfakes.FakeWorker
		fakeStrategy *workerfakes.FakeContainerPlacementStrategy

		fakeResourceFactory      *resourcefakes.FakeResourceFactory
		fakeResource             *resourcefakes.FakeResource
		fakeResourceCacheFactory *dbfakes.FakeResourceCacheFactory
		fakeResourceCache        *dbfakes.FakeUsedResourceCache

		fakeDelegate        *execfakes.FakeGetDelegate
		fakeDelegateFactory *execfakes.FakeGetDelegateFactory

		spanCtx context.Context

		getPlan *atc.GetPlan

		state exec.RunState

		getStep    exec.Step
		getStepOk  bool
		getStepErr error

		containerMetadata = db.ContainerMetadata{
			WorkingDirectory: resource.ResourcesDir("get"),
			PipelineID:       4567,
			Type:             db.ContainerTypeGet,
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

		planID atc.PlanID = "56"
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		fakeClient = new(workerfakes.FakeClient)
		fakeWorker = new(workerfakes.FakeWorker)
		fakeWorker.NameReturns("some-worker")
		fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)

		fakeResourceFactory = new(resourcefakes.FakeResourceFactory)
		fakeResource = new(resourcefakes.FakeResource)
		fakeResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)
		fakeResourceCache = new(dbfakes.FakeUsedResourceCache)

		credVars := vars.StaticVariables{
			"source-var": "super-secret-source",
			"params-var": "super-secret-params",
		}
		state = exec.NewRunState(noopStepper, credVars, false)

		fakeDelegate = new(execfakes.FakeGetDelegate)
		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
		fakeDelegate.StdoutReturns(stdoutBuf)
		fakeDelegate.StderrReturns(stderrBuf)
		spanCtx = context.Background()
		fakeDelegate.StartSpanReturns(spanCtx, trace.NoopSpan{})

		fakeDelegateFactory = new(execfakes.FakeGetDelegateFactory)
		fakeDelegateFactory.GetDelegateReturns(fakeDelegate)

		getPlan = &atc.GetPlan{
			Name:    "some-name",
			Type:    "some-base-type",
			Source:  atc.Source{"some": "((source-var))"},
			Params:  atc.Params{"some": "((params-var))"},
			Version: &atc.Version{"some": "version"},
			VersionedResourceTypes: atc.VersionedResourceTypes{
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
			},
		}
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		plan := atc.Plan{
			ID:  atc.PlanID(planID),
			Get: getPlan,
		}

		fakeResourceCacheFactory.FindOrCreateResourceCacheReturns(fakeResourceCache, nil)
		fakeResourceFactory.NewResourceReturns(fakeResource)

		getStep = exec.NewGetStep(
			plan.ID,
			*plan.Get,
			stepMetadata,
			containerMetadata,
			fakeResourceFactory,
			fakeResourceCacheFactory,
			fakeStrategy,
			fakeDelegateFactory,
			fakeClient,
		)

		getStepOk, getStepErr = getStep.Run(ctx, state)
	})

	It("propagates span context to the worker client", func() {
		actualCtx, _, _, _, _, _, _, _, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualCtx).To(Equal(spanCtx))
	})

	It("constructs the resource cache correctly", func() {
		_, typ, ver, source, params, types := fakeResourceCacheFactory.FindOrCreateResourceCacheArgsForCall(0)
		Expect(typ).To(Equal("some-base-type"))
		Expect(ver).To(Equal(atc.Version{"some": "version"}))
		Expect(source).To(Equal(atc.Source{"some": "super-secret-source"}))
		Expect(params).To(Equal(atc.Params{"some": "super-secret-params"}))
		Expect(types).To(Equal(atc.VersionedResourceTypes{
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
		}))
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
			actualCtx, _, _, _, _, _, _, _, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
			Expect(actualCtx).To(Equal(spanCtx))
		})

		It("populates the TRACEPARENT env var", func() {
			_, _, _, actualContainerSpec, _, _, _, _, _, _, _ := fakeClient.RunGetStepArgsForCall(0)

			Expect(actualContainerSpec.Env).To(ContainElement(MatchRegexp(`TRACEPARENT=.+`)))
		})
	})

	It("calls RunGetStep with the correct ContainerOwner", func() {
		_, _, actualContainerOwner, _, _, _, _, _, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualContainerOwner).To(Equal(db.NewBuildStepContainerOwner(
			stepMetadata.BuildID,
			atc.PlanID(planID),
			stepMetadata.TeamID,
		)))
	})

	It("calls RunGetStep with the correct ContainerSpec", func() {
		_, _, _, actualContainerSpec, _, _, _, _, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualContainerSpec).To(Equal(
			worker.ContainerSpec{
				ImageSpec: worker.ImageSpec{
					ResourceType: "some-base-type",
				},
				TeamID: stepMetadata.TeamID,
				Env:    stepMetadata.Env(),
			},
		))
	})

	It("calls RunGetStep with the correct WorkerSpec", func() {
		_, _, _, _, actualWorkerSpec, _, _, _, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualWorkerSpec).To(Equal(
			worker.WorkerSpec{
				ResourceType: "some-base-type",
				TeamID:       stepMetadata.TeamID,
			},
		))
	})

	Context("when the plan specifies tags", func() {
		BeforeEach(func() {
			getPlan.Tags = atc.Tags{"some", "tags"}
		})

		It("sets them in the WorkerSpec", func() {
			_, _, _, _, actualWorkerSpec, _, _, _, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
			Expect(actualWorkerSpec.Tags).To(Equal([]string{"some", "tags"}))
		})
	})

	Context("when using a custom resource type", func() {
		var fakeImageSpec worker.ImageSpec

		BeforeEach(func() {
			getPlan.Type = "some-custom-type"

			fakeImageSpec = worker.ImageSpec{
				ImageArtifact: new(runtimefakes.FakeArtifact),
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

		Context("when the plan configures tags", func() {
			BeforeEach(func() {
				getPlan.Tags = atc.Tags{"plan", "tags"}
			})

			It("fetches using the tags", func() {
				Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
				_, imageResource, _, _ := fakeDelegate.FetchImageArgsForCall(0)
				Expect(imageResource.Tags).To(Equal(atc.Tags{"plan", "tags"}))
			})
		})

		Context("when the resource type configures tags", func() {
			BeforeEach(func() {
				taggedType, found := getPlan.VersionedResourceTypes.Lookup("some-custom-type")
				Expect(found).To(BeTrue())

				taggedType.Tags = atc.Tags{"type", "tags"}

				newTypes := getPlan.VersionedResourceTypes.Without("some-custom-type")
				newTypes = append(newTypes, taggedType)

				getPlan.VersionedResourceTypes = newTypes
			})

			It("fetches using the type tags", func() {
				Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
				_, imageResource, _, _ := fakeDelegate.FetchImageArgsForCall(0)
				Expect(imageResource.Tags).To(Equal(atc.Tags{"type", "tags"}))
			})

			Context("when the plan ALSO configures tags", func() {
				BeforeEach(func() {
					getPlan.Tags = atc.Tags{"plan", "tags"}
				})

				It("fetches using only the type tags", func() {
					Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
					_, imageResource, _, _ := fakeDelegate.FetchImageArgsForCall(0)
					Expect(imageResource.Tags).To(Equal(atc.Tags{"type", "tags"}))
				})
			})
		})

		It("sets the bottom-most type in the worker spec", func() {
			_, _, _, _, actualWorkerSpec, _, _, _, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
			Expect(actualWorkerSpec).To(Equal(
				worker.WorkerSpec{
					TeamID:       stepMetadata.TeamID,
					ResourceType: "registry-image",
				},
			))
		})

		It("calls RunGetStep with the correct ImageSpec", func() {
			_, _, _, containerSpec, _, _, _, _, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
			Expect(containerSpec.ImageSpec).To(Equal(fakeImageSpec))
		})

		Context("when the resource type is privileged", func() {
			BeforeEach(func() {
				getPlan.Type = "another-custom-type"
			})

			It("fetches the image with privileged", func() {
				Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
				_, _, _, privileged := fakeDelegate.FetchImageArgsForCall(0)
				Expect(privileged).To(BeTrue())
			})
		})
	})

	It("calls RunGetStep with the correct ContainerPlacementStrategy", func() {
		_, _, _, _, _, actualStrategy, _, _, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualStrategy).To(Equal(fakeStrategy))
	})

	It("calls RunGetStep with the correct ContainerMetadata", func() {
		_, _, _, _, _, _, actualContainerMetadata, _, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualContainerMetadata).To(Equal(
			db.ContainerMetadata{
				PipelineID:       4567,
				Type:             db.ContainerTypeGet,
				StepName:         "some-step",
				WorkingDirectory: "/tmp/build/get",
			},
		))
	})

	It("calls RunGetStep with the correct StartingEventDelegate", func() {
		_, _, _, _, _, _, _, _, actualEventDelegate, _, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualEventDelegate).To(Equal(fakeDelegate))
	})

	It("calls RunGetStep with the correct ProcessSpec", func() {
		_, _, _, _, _, _, _, actualProcessSpec, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualProcessSpec).To(Equal(
			runtime.ProcessSpec{
				Path:         "/opt/resource/in",
				Args:         []string{resource.ResourcesDir("get")},
				StdoutWriter: fakeDelegate.Stdout(),
				StderrWriter: fakeDelegate.Stderr(),
			},
		))
	})

	It("calls RunGetStep with the correct ResourceCache", func() {
		_, _, _, _, _, _, _, _, _, actualResourceCache, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualResourceCache).To(Equal(fakeResourceCache))
	})

	It("calls RunGetStep with the correct Resource", func() {
		_, _, _, _, _, _, _, _, _, _, actualResource := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualResource).To(Equal(fakeResource))
	})

	Context("when Client.RunGetStep returns an err", func() {
		var disaster error
		BeforeEach(func() {
			disaster = errors.New("disaster")
			fakeClient.RunGetStepReturns(worker.GetResult{}, disaster)
		})
		It("returns an err", func() {
			Expect(fakeClient.RunGetStepCallCount()).To(Equal(1))
			Expect(getStepErr).To(HaveOccurred())
			Expect(getStepErr).To(Equal(disaster))
		})
	})

	Context("when Client.RunGetStep returns a Successful GetResult", func() {
		BeforeEach(func() {
			fakeClient.RunGetStepReturns(
				worker.GetResult{
					ExitStatus: 0,
					VersionResult: runtime.VersionResult{
						Version:  atc.Version{"some": "version"},
						Metadata: []atc.MetadataField{{Name: "some", Value: "metadata"}},
					},
					GetArtifact: runtime.GetArtifact{VolumeHandle: "some-volume-handle"},
				}, nil)
		})

		It("registers the resulting artifact in the RunState.ArtifactRepository", func() {
			artifact, found := state.ArtifactRepository().ArtifactFor(build.ArtifactName(getPlan.Name))
			Expect(artifact).To(Equal(runtime.GetArtifact{VolumeHandle: "some-volume-handle"}))
			Expect(found).To(BeTrue())
		})

		It("stores the resource cache as the step result", func() {
			var result interface{}
			Expect(state.Result(planID, &result)).To(BeTrue())
			Expect(result).To(Equal(fakeResourceCache))
		})

		It("marks the step as succeeded", func() {
			Expect(getStepOk).To(BeTrue())
		})

		It("finishes the step via the delegate", func() {
			Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
			_, status, info := fakeDelegate.FinishedArgsForCall(0)
			Expect(status).To(Equal(exec.ExitStatus(0)))
			Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
			Expect(info.Metadata).To(Equal([]atc.MetadataField{{Name: "some", Value: "metadata"}}))
		})

		Context("when the plan has a resource", func() {
			BeforeEach(func() {
				getPlan.Resource = "some-pipeline-resource"
			})

			It("saves a version for the resource", func() {
				Expect(fakeDelegate.UpdateVersionCallCount()).To(Equal(1))
				_, actualPlan, actualVersionResult := fakeDelegate.UpdateVersionArgsForCall(0)
				Expect(actualPlan.Resource).To(Equal("some-pipeline-resource"))
				Expect(actualVersionResult.Version).To(Equal(atc.Version{"some": "version"}))
				Expect(actualVersionResult.Metadata).To(Equal([]atc.MetadataField{{Name: "some", Value: "metadata"}}))
			})
		})

		Context("when getting an anonymous resource", func() {
			BeforeEach(func() {
				getPlan.Resource = ""
			})

			It("does not save the version", func() {
				Expect(fakeDelegate.UpdateVersionCallCount()).To(Equal(0))
			})
		})

		It("does not return an err", func() {
			Expect(getStepErr).ToNot(HaveOccurred())
		})
	})

	Context("when Client.RunGetStep returns a Failed GetResult", func() {
		BeforeEach(func() {
			fakeClient.RunGetStepReturns(
				worker.GetResult{
					ExitStatus:    1,
					VersionResult: runtime.VersionResult{},
				}, nil)
		})

		It("does NOT mark the step as succeeded", func() {
			Expect(getStepOk).To(BeFalse())
		})

		It("finishes the step via the delegate", func() {
			Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
			_, actualExitStatus, actualVersionResult := fakeDelegate.FinishedArgsForCall(0)
			Expect(actualExitStatus).ToNot(Equal(exec.ExitStatus(0)))
			Expect(actualVersionResult).To(Equal(runtime.VersionResult{}))
		})

		It("does not return an err", func() {
			Expect(getStepErr).ToNot(HaveOccurred())
		})
	})
})
