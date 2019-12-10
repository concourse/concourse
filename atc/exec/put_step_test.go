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
		fakeClient                *workerfakes.FakeClient
		fakeStrategy              *workerfakes.FakeContainerPlacementStrategy
		fakeResourceFactory       *resourcefakes.FakeResourceFactory
		fakeResource              *resourcefakes.FakeResource
		fakeResourceConfigFactory *dbfakes.FakeResourceConfigFactory
		fakeDelegate              *execfakes.FakePutDelegate
		putPlan                   *atc.PutPlan

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

		versionResult = runtime.VersionResult{
			Version:  atc.Version{"some": "version"},
			Metadata: []atc.MetadataField{{Name: "some", Value: "metadata"}},
		}

		fakeResource = new(resourcefakes.FakeResource)
		fakeResource.PutReturns(versionResult, nil)

		repo = build.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactRepositoryReturns(repo)

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

		fakeArtifact = new(runtimefakes.FakeArtifact)
		fakeOtherArtifact = new(runtimefakes.FakeArtifact)
		fakeMountedArtifact = new(runtimefakes.FakeArtifact)

		repo.RegisterArtifact("some-source", fakeArtifact)
		repo.RegisterArtifact("some-other-source", fakeOtherArtifact)
		repo.RegisterArtifact("some-mounted-source", fakeMountedArtifact)

		fakeResourceFactory.NewResourceReturns(fakeResource)

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

	Context("inputs", func() {
		Context("when inputs are specified with 'all' keyword", func() {
			BeforeEach(func() {
				putPlan.Inputs = &atc.InputsConfig{
					All: true,
				}
			})

			It("calls RunPutStep with all inputs", func() {
				_, _, _, actualContainerSpec, _, _, _, _, _, _ := fakeClient.RunPutStepArgsForCall(0)
				Expect(actualContainerSpec.ArtifactByPath).To(HaveLen(3))
				Expect(actualContainerSpec.ArtifactByPath["/tmp/build/put/some-other-source"]).To(Equal(fakeOtherArtifact))
				Expect(actualContainerSpec.ArtifactByPath["/tmp/build/put/some-mounted-source"]).To(Equal(fakeMountedArtifact))
				Expect(actualContainerSpec.ArtifactByPath["/tmp/build/put/some-source"]).To(Equal(fakeArtifact))
			})
		})

		Context("when inputs are left blank", func() {
			It("calls RunPutStep with all inputs", func() {
				_, _, _, actualContainerSpec, _, _, _, _, _, _ := fakeClient.RunPutStepArgsForCall(0)
				Expect(actualContainerSpec.ArtifactByPath).To(HaveLen(3))
				Expect(actualContainerSpec.ArtifactByPath["/tmp/build/put/some-other-source"]).To(Equal(fakeOtherArtifact))
				Expect(actualContainerSpec.ArtifactByPath["/tmp/build/put/some-mounted-source"]).To(Equal(fakeMountedArtifact))
				Expect(actualContainerSpec.ArtifactByPath["/tmp/build/put/some-source"]).To(Equal(fakeArtifact))
			})
		})

		Context("when only some inputs are specified ", func() {
			BeforeEach(func() {
				putPlan.Inputs = &atc.InputsConfig{
					Specified: []string{"some-source", "some-other-source"},
				}
			})

			It("calls RunPutStep with specified inputs", func() {
				_, _, _, containerSpec, _, _, _, _, _, _ := fakeClient.RunPutStepArgsForCall(0)
				Expect(containerSpec.ArtifactByPath).To(HaveLen(2))
				Expect(containerSpec.ArtifactByPath["/tmp/build/put/some-other-source"]).To(Equal(fakeOtherArtifact))
				Expect(containerSpec.ArtifactByPath["/tmp/build/put/some-source"]).To(Equal(fakeArtifact))
			})
		})
	})

	It("calls workerClient -> RunPutStep with the appropriate arguments", func() {
		Expect(fakeClient.RunPutStepCallCount()).To(Equal(1))
		actualContext, _, actualOwner, actualContainerSpec, actualWorkerSpec, actualStrategy, actualContainerMetadata, actualImageFetcherSpec, actualProcessSpec, actualResource := fakeClient.RunPutStepArgsForCall(0)

		Expect(actualContext).To(Equal(ctx))
		Expect(actualOwner).To(Equal(db.NewBuildStepContainerOwner(42, atc.PlanID(planID), 123)))
		Expect(actualContainerSpec.ImageSpec).To(Equal(worker.ImageSpec{
			ResourceType: "some-resource-type",
		}))
		Expect(actualContainerSpec.Tags).To(Equal([]string{"some", "tags"}))
		Expect(actualContainerSpec.TeamID).To(Equal(123))
		Expect(actualContainerSpec.Env).To(Equal(stepMetadata.Env()))
		Expect(actualContainerSpec.Dir).To(Equal("/tmp/build/put"))

		Expect(actualContainerSpec.ArtifactByPath).To(HaveLen(3))
		Expect(actualContainerSpec.ArtifactByPath["/tmp/build/put/some-other-source"]).To(Equal(fakeOtherArtifact))
		Expect(actualContainerSpec.ArtifactByPath["/tmp/build/put/some-mounted-source"]).To(Equal(fakeMountedArtifact))
		Expect(actualContainerSpec.ArtifactByPath["/tmp/build/put/some-source"]).To(Equal(fakeArtifact))

		Expect(actualWorkerSpec).To(Equal(worker.WorkerSpec{
			TeamID:        123,
			Tags:          []string{"some", "tags"},
			ResourceType:  "some-resource-type",
			ResourceTypes: interpolatedResourceTypes,
		}))
		Expect(actualStrategy).To(Equal(fakeStrategy))

		Expect(actualContainerMetadata).To(Equal(containerMetadata))
		Expect(actualImageFetcherSpec.ResourceTypes).To(Equal(interpolatedResourceTypes))
		Expect(actualImageFetcherSpec.Delegate).To(Equal(fakeDelegate))

		Expect(actualProcessSpec).To(Equal(
			runtime.ProcessSpec{
				Path:         "/opt/resource/out",
				Args:         []string{resource.ResourcesDir("put")},
				StdoutWriter: stdoutBuf,
				StderrWriter: stderrBuf,
			}))
		Expect(actualResource).To(Equal(fakeResource))
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

		It("secrets are tracked", func() {
			mapit := vars.NewMapCredVarsTrackerIterator()
			credVarsTracker.IterateInterpolatedCreds(mapit)
			Expect(mapit.Data["custom-param"]).To(Equal("source"))
			Expect(mapit.Data["source-param"]).To(Equal("super-secret-source"))
		})

		It("creates a resource with the correct source and params", func() {
			actualSource, actualParams, _ := fakeResourceFactory.NewResourceArgsForCall(0)
			Expect(actualSource).To(Equal(atc.Source{"some": "super-secret-source"}))
			Expect(actualParams).To(Equal(atc.Params{"some-param": "some-value"}))

			_, _, _, _, _, _, _, _, _, actualResource := fakeClient.RunPutStepArgsForCall(0)
			Expect(actualResource).To(Equal(fakeResource))
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
		Expect(info.Metadata).To(Equal([]atc.MetadataField{{"some", "metadata"}}))
	})

	Context("when the step.Plan.Resource is blank", func() {
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
			Expect(putStep.Succeeded()).To(BeTrue())
		})
	})

	Context("when RunPutStep exits unsuccessfully", func() {
		BeforeEach(func() {
			versionResult = runtime.VersionResult{}
			clientErr = runtime.ErrResourceScriptFailed{
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
