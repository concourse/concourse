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

var _ = Describe("GetStep", func() {
	var (
		ctx       context.Context
		cancel    func()
		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		fakeClient   *workerfakes.FakeClient
		fakeWorker   *workerfakes.FakeWorker
		fakePool     *workerfakes.FakePool
		fakeStrategy *workerfakes.FakeContainerPlacementStrategy

		fakeResourceFactory      *resourcefakes.FakeResourceFactory
		fakeResource             *resourcefakes.FakeResource
		fakeResourceCacheFactory *dbfakes.FakeResourceCacheFactory
		fakeResourceCache        *dbfakes.FakeUsedResourceCache

		fakeDelegate *execfakes.FakeGetDelegate

		getPlan *atc.GetPlan

		interpolatedResourceTypes atc.VersionedResourceTypes

		artifactRepository *build.Repository
		fakeState          *execfakes.FakeRunState

		getStep    exec.Step
		getStepErr error

		credVarsTracker vars.CredVarsTracker

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

		planID = 56
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		fakeClient = new(workerfakes.FakeClient)
		fakeWorker = new(workerfakes.FakeWorker)
		fakeWorker.NameReturns("some-worker")
		fakePool = new(workerfakes.FakePool)
		fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)

		fakeResourceFactory = new(resourcefakes.FakeResourceFactory)
		fakeResource = new(resourcefakes.FakeResource)
		fakeResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)
		fakeResourceCache = new(dbfakes.FakeUsedResourceCache)

		credVars := vars.StaticVariables{"source-param": "super-secret-source"}
		credVarsTracker = vars.NewCredVarsTracker(credVars, true)

		artifactRepository = build.NewRepository()
		fakeState = new(execfakes.FakeRunState)

		fakeState.ArtifactRepositoryReturns(artifactRepository)

		fakeDelegate = new(execfakes.FakeGetDelegate)
		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
		fakeDelegate.VariablesReturns(credVarsTracker)
		fakeDelegate.StdoutReturns(stdoutBuf)
		fakeDelegate.StderrReturns(stderrBuf)

		uninterpolatedResourceTypes := atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-resource",
					Type:   "custom-type",
					Source: atc.Source{"some-custom": "((source-param))"},
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
				},
				Version: atc.Version{"some-custom": "version"},
			},
		}

		getPlan = &atc.GetPlan{
			Name:                   "some-name",
			Type:                   "some-resource-type",
			Source:                 atc.Source{"some": "((source-param))"},
			Params:                 atc.Params{"some-param": "some-value"},
			Tags:                   []string{"some", "tags"},
			Version:                &atc.Version{"some-version": "some-value"},
			VersionedResourceTypes: uninterpolatedResourceTypes,
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
			fakePool,
			fakeDelegate,
			fakeClient,
		)

		getStepErr = getStep.Run(ctx, fakeState)
	})

	It("calls RunGetStep with the correct ctx", func() {
		actualCtx, _, _, _, _, _, _, _, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualCtx).To(Equal(ctx))
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
					ResourceType: "some-resource-type",
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
				ResourceType:  "some-resource-type",
				Tags:          atc.Tags{"some", "tags"},
				TeamID:        stepMetadata.TeamID,
				ResourceTypes: interpolatedResourceTypes,
			},
		))
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

	It("calls RunGetStep with the correct ImageFetcherSpec", func() {
		_, _, _, _, _, _, _, actualImageFetcherSpec, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualImageFetcherSpec).To(Equal(
			worker.ImageFetcherSpec{
				ResourceTypes: interpolatedResourceTypes,
				Delegate:      fakeDelegate,
			},
		))
	})

	It("calls RunGetStep with the correct ProcessSpec", func() {
		_, _, _, _, _, _, _, _, actualProcessSpec, _, _ := fakeClient.RunGetStepArgsForCall(0)
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
					Status: 0,
					VersionResult: runtime.VersionResult{
						Version:  atc.Version{"some": "version"},
						Metadata: []atc.MetadataField{{Name: "some", Value: "metadata"}},
					},
					GetArtifact: runtime.GetArtifact{VolumeHandle: "some-volume-handle"},
				}, nil)
		})

		It("registers the resulting artifact in the RunState.ArtifactRepository", func() {
			artifact, found := artifactRepository.ArtifactFor(build.ArtifactName(getPlan.Name))
			Expect(artifact).To(Equal(runtime.GetArtifact{"some-volume-handle"}))
			Expect(found).To(BeTrue())
		})

		It("marks the step as succeeded", func() {
			Expect(getStep.Succeeded()).To(BeTrue())
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
					Status:        1,
					VersionResult: runtime.VersionResult{},
				}, nil)
		})

		It("does NOT mark the step as succeeded", func() {
			Expect(getStep.Succeeded()).To(BeFalse())
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
