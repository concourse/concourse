package exec_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/resource/resourcefakes"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/concourse/concourse/vars"
)

var _ = Describe("PutStep", func() {
	var (
		ctx    context.Context
		cancel func()

		fakeWorker                *workerfakes.FakeWorker
		fakeClient                *workerfakes.FakeClient
		fakeStrategy              *workerfakes.FakeContainerPlacementStrategy
		fakeResourceFactory       *resourcefakes.FakeResourceFactory
		fakeResourceConfigFactory *dbfakes.FakeResourceConfigFactory
		fakeDelegate              *execfakes.FakePutDelegate
		putPlan                   *atc.PutPlan

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

		putStep *exec.PutStep
		stepErr error

		credVarsTracker vars.CredVarsTracker

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		planID atc.PlanID

		versionResult runtime.VersionResult
		clientErr     error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		planID = atc.PlanID("some-plan-id")

		fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)
		fakeClient = new(workerfakes.FakeClient)
		fakeWorker = new(workerfakes.FakeWorker)
		fakeResourceFactory = new(resourcefakes.FakeResourceFactory)
		fakeResourceConfigFactory = new(dbfakes.FakeResourceConfigFactory)

		credVars := vars.StaticVariables{"custom-param": "source", "source-param": "super-secret-source"}
		credVarsTracker = vars.NewCredVarsTracker(credVars, true)

		fakeDelegate = new(execfakes.FakePutDelegate)
		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
		fakeDelegate.StdoutReturns(stdoutBuf)
		fakeDelegate.StderrReturns(stderrBuf)
		fakeDelegate.VariablesReturns(vars.NewCredVarsTracker(credVarsTracker, false))

		repo = build.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactsReturns(repo)

		uninterpolatedResourceTypes := atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-resource",
					Type:   "custom-type",
					Source: atc.Source{"some-custom": "((custom-param))"},
				},
				Version: atc.Version{"some-custom": "version"},
			},
		}

		interpolatedResourceTypes = atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-resource",
					Type:   "custom-type",
					Source: atc.Source{"some-custom": "source"},
				},
				Version: atc.Version{"some-custom": "version"},
			},
		}

		putPlan = &atc.PutPlan{
			Name:                   "some-name",
			Resource:               "some-resource",
			Type:                   "some-resource-type",
			Source:                 atc.Source{"some": "((source-param))"},
			Params:                 atc.Params{"some-param": "some-value"},
			Tags:                   []string{"some", "tags"},
			VersionedResourceTypes: uninterpolatedResourceTypes,
		}

		versionResult = runtime.VersionResult{
			Version:  atc.Version{"some": "version"},
			Metadata: []atc.MetadataField{{Name: "some", Value: "metadata"}},
		}
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		plan := atc.Plan{
			ID:  atc.PlanID(planID),
			Put: putPlan,
		}

		fakeClient.RunPutStepReturns(worker.PutResult{Status: 0, VersionResult: versionResult, Err: clientErr})

		putStep = exec.NewPutStep(
			plan.ID,
			*plan.Put,
			stepMetadata,
			containerMetadata,
			fakeResourceFactory,
			fakeResourceConfigFactory,
			fakeStrategy,
			fakeClient,
			fakeDelegate,
		)

		stepErr = putStep.Run(ctx, state)
	})

	Context("when repo contains sources", func() {
		var (
			fakeSource        *workerfakes.FakeArtifactSource
			fakeOtherSource   *workerfakes.FakeArtifactSource
			fakeMountedSource *workerfakes.FakeArtifactSource
		)

		BeforeEach(func() {
			fakeSource = new(workerfakes.FakeArtifactSource)
			fakeOtherSource = new(workerfakes.FakeArtifactSource)
			fakeMountedSource = new(workerfakes.FakeArtifactSource)

			repo.RegisterSource("some-source", fakeSource)
			repo.RegisterSource("some-other-source", fakeOtherSource)
			repo.RegisterSource("some-mounted-source", fakeMountedSource)

		})

		It("finds/chooses a worker and creates a container with the correct type, session, and sources with no inputs specified (meaning it takes all artifacts)", func() {
			Expect(fakeClient.RunPutStepCallCount()).To(Equal(1))
			_, _, actualOwner, actualContainerSpec, actualWorkerSpec, _, _, strategy, _, actualImageFetcherSpec, _, _, _ := fakeClient.RunPutStepArgsForCall(0)

			Expect(actualOwner).To(Equal(db.NewBuildStepContainerOwner(42, atc.PlanID(planID), 123)))
			Expect(actualContainerSpec.ImageSpec).To(Equal(worker.ImageSpec{
				ResourceType: "some-resource-type",
			}))
			Expect(actualContainerSpec.Tags).To(Equal([]string{"some", "tags"}))
			Expect(actualContainerSpec.TeamID).To(Equal(123))
			Expect(actualContainerSpec.Env).To(Equal(stepMetadata.Env()))
			Expect(actualContainerSpec.Dir).To(Equal("/tmp/build/put"))
			Expect(actualContainerSpec.Inputs).To(HaveLen(3))
			Expect(actualWorkerSpec).To(Equal(worker.WorkerSpec{
				TeamID:        123,
				Tags:          []string{"some", "tags"},
				ResourceType:  "some-resource-type",
				ResourceTypes: interpolatedResourceTypes,
			}))
			Expect(strategy).To(Equal(fakeStrategy))

			Expect([]worker.ArtifactSource{
				actualContainerSpec.Inputs[0].Source(),
				actualContainerSpec.Inputs[1].Source(),
				actualContainerSpec.Inputs[2].Source(),
			}).To(ConsistOf(
				exec.PutResourceSource{fakeSource},
				exec.PutResourceSource{fakeOtherSource},
				exec.PutResourceSource{fakeMountedSource},
			))
			Expect(actualImageFetcherSpec.ResourceTypes).To(Equal(interpolatedResourceTypes))
			Expect(actualImageFetcherSpec.Delegate).To(Equal(fakeDelegate))
		})

		Context("when the tracker can initialize the resource", func() {
			var (
				fakeResource       *resourcefakes.FakeResource
				fakeResourceConfig *dbfakes.FakeResourceConfig
				fakeVersionResult  runtime.VersionResult
			)

			BeforeEach(func() {
				fakeResourceConfig = new(dbfakes.FakeResourceConfig)
				fakeResourceConfig.IDReturns(1)

				fakeResourceConfigFactory.FindOrCreateResourceConfigReturns(fakeResourceConfig, nil)

				fakeVersionResult = runtime.VersionResult{
					Version:  atc.Version{"some": "version"},
					Metadata: []atc.MetadataField{{Name: "some", Value: "metadata"}},
				}

				fakeWorker.NameReturns("some-worker")

				fakeResource = new(resourcefakes.FakeResource)
				fakeResource.PutReturns(fakeVersionResult, nil)
				fakeResourceFactory.NewResourceForContainerReturns(fakeResource)
			})

			It("secrets are tracked", func() {
				mapit := vars.NewMapCredVarsTrackerIterator()
				credVarsTracker.IterateInterpolatedCreds(mapit)
				Expect(mapit.Data["custom-param"]).To(Equal("source"))
				Expect(mapit.Data["source-param"]).To(Equal("super-secret-source"))
			})

			Context("when the inputs are specified", func() {
				BeforeEach(func() {
					putPlan.Inputs = &atc.InputsConfig{
						Specified: []string{"some-source", "some-other-source"},
					}
				})

				It("calls RunPutStep with specified inputs", func() {
					_, _, _, containerSpec, _, _, _, _, _, _, _, _, _ := fakeClient.RunPutStepArgsForCall(0)
					Expect(containerSpec.Inputs).To(HaveLen(2))
					Expect([]worker.ArtifactSource{
						containerSpec.Inputs[0].Source(),
						containerSpec.Inputs[1].Source(),
					}).To(ConsistOf(
						exec.PutResourceSource{fakeSource},
						exec.PutResourceSource{fakeOtherSource},
					))
				})
			})

			It("calls RunPutStep with the given context", func() {
				Expect(fakeClient.RunPutStepCallCount()).To(Equal(1))
				putCtx, _, _, _, _, _, _, _, _, _, _, _, _ := fakeClient.RunPutStepArgsForCall(0)
				Expect(putCtx).To(Equal(ctx))
			})

			It("puts the resource with the correct source and params", func() {
				Expect(fakeClient.RunPutStepCallCount()).To(Equal(1))
				_, _, _, _, _, putSource, putParams, _, _, _, _, _, _ := fakeClient.RunPutStepArgsForCall(0)
				Expect(putSource).To(Equal(atc.Source{"some": "super-secret-source"}))
				Expect(putParams).To(Equal(atc.Params{"some-param": "some-value"}))
			})

			It("puts the resource with the io config forwarded", func() {
				Expect(fakeClient.RunPutStepCallCount()).To(Equal(1))
				_, _, _, _, _, _, _, _, _, _, _, processSpec, _ := fakeClient.RunPutStepArgsForCall(0)
				Expect(processSpec.StdoutWriter).To(Equal(stdoutBuf))
				Expect(processSpec.StderrWriter).To(Equal(stderrBuf))
			})

			It("is successful", func() {
				Expect(putStep.Succeeded()).To(BeTrue())
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
				Expect(info.Metadata).To(Equal([]atc.MetadataField{{"some", "metadata"}}))
			})

			Context("when the resource is blank", func() {
				BeforeEach(func() {
					putPlan.Resource = ""
				})

				It("is successful", func() {
					Expect(putStep.Succeeded()).To(BeTrue())
				})

				It("does not save the build output", func() {
					Expect(fakeDelegate.SaveOutputCallCount()).To(Equal(0))
				})
			})

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

			Context("when RunPutStep exits unsuccessfully", func() {
				BeforeEach(func() {
					versionResult = runtime.VersionResult{}
					clientErr = resource.ErrResourceScriptFailed{
						ExitStatus: 42,
					}
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
					Expect(putStep.Succeeded()).To(BeFalse())
				})
			})

			Context("when RunPutStep exits with an error", func() {
				disaster := errors.New("oh no")

				BeforeEach(func() {
					versionResult = runtime.VersionResult{}
					clientErr = disaster
				})

				It("does not finish the step via the delegate", func() {
					Expect(fakeDelegate.FinishedCallCount()).To(Equal(0))
				})

				It("returns the error", func() {
					Expect(stepErr).To(Equal(disaster))
				})

				It("is not successful", func() {
					Expect(putStep.Succeeded()).To(BeFalse())
				})
			})
		})
	})
})
