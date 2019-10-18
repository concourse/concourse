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
	"github.com/concourse/concourse/atc/exec/artifact"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/resource/resourcefakes"
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

		repo  *artifact.Repository
		state *execfakes.FakeRunState

		putStep *exec.PutStep
		stepErr error

		credVarsTracker vars.CredVarsTracker

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		planID atc.PlanID
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		planID = atc.PlanID("some-plan-id")

		fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)
		fakePool = new(workerfakes.FakePool)
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

		repo = artifact.NewRepository()
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

		Context("when the tracker can initialize the resource", func() {
			var (
				fakeResource       *resourcefakes.FakeResource
				fakeResourceConfig *dbfakes.FakeResourceConfig
				fakeVersionResult  resource.VersionResult
			)

			BeforeEach(func() {
				fakeResourceConfig = new(dbfakes.FakeResourceConfig)
				fakeResourceConfig.IDReturns(1)

				fakeResourceConfigFactory.FindOrCreateResourceConfigReturns(fakeResourceConfig, nil)

				fakeVersionResult = resource.VersionResult{
					Version:  atc.Version{"some": "version"},
					Metadata: []atc.MetadataField{{Name: "some", Value: "metadata"}},
				}

				fakeWorker.NameReturns("some-worker")
				fakePool.FindOrChooseWorkerForContainerReturns(fakeWorker, nil)

				fakeResource = new(resourcefakes.FakeResource)
				fakeResource.PutReturns(fakeVersionResult, nil)
				fakeResourceFactory.NewResourceForContainerReturns(fakeResource)
			})

			It("finds/chooses a worker and creates a container with the correct type, session, and sources with no inputs specified (meaning it takes all artifacts)", func() {
				Expect(fakePool.FindOrChooseWorkerForContainerCallCount()).To(Equal(1))
				_, _, actualOwner, actualContainerSpec, actualWorkerSpec, strategy := fakePool.FindOrChooseWorkerForContainerArgsForCall(0)
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

				_, _, delegate, owner, actualContainerMetadata, containerSpec, actualResourceTypes := fakeWorker.FindOrCreateContainerArgsForCall(0)
				Expect(owner).To(Equal(db.NewBuildStepContainerOwner(42, atc.PlanID(planID), 123)))
				Expect(actualContainerMetadata).To(Equal(containerMetadata))
				Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
					ResourceType: "some-resource-type",
				}))
				Expect(containerSpec.Tags).To(Equal([]string{"some", "tags"}))
				Expect(containerSpec.TeamID).To(Equal(123))
				Expect(containerSpec.Env).To(Equal(stepMetadata.Env()))
				Expect(containerSpec.Dir).To(Equal("/tmp/build/put"))
				Expect(containerSpec.Inputs).To(HaveLen(3))

				Expect([]worker.ArtifactSource{
					containerSpec.Inputs[0].Source(),
					containerSpec.Inputs[1].Source(),
					containerSpec.Inputs[2].Source(),
				}).To(ConsistOf(
					exec.PutResourceSource{fakeSource},
					exec.PutResourceSource{fakeOtherSource},
					exec.PutResourceSource{fakeMountedSource},
				))
				Expect(actualResourceTypes).To(Equal(interpolatedResourceTypes))
				Expect(delegate).To(Equal(fakeDelegate))
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

				It("initializes the container with specified inputs", func() {
					_, _, _, _, _, containerSpec, _ := fakeWorker.FindOrCreateContainerArgsForCall(0)
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

			Context("when the inputs are detected", func() {
				BeforeEach(func() {
					putPlan.Inputs = &atc.InputsConfig{
						Detect: true,
					}
					putPlan.Params = atc.Params{"some-param": "some-source/source", "another-param": "does-not-exist", "number-param": 123}
				})

				It("initializes the container with detected inputs", func() {
					_, _, _, _, _, containerSpec, _ := fakeWorker.FindOrCreateContainerArgsForCall(0)
					Expect(containerSpec.Inputs).To(HaveLen(1))
					Expect([]worker.ArtifactSource{
						containerSpec.Inputs[0].Source(),
					}).To(ConsistOf(
						exec.PutResourceSource{fakeSource},
					))
				})
			})

			It("puts the resource with the given context", func() {
				Expect(fakeResource.PutCallCount()).To(Equal(1))
				putCtx, _, _, _ := fakeResource.PutArgsForCall(0)
				Expect(putCtx).To(Equal(ctx))
			})

			It("puts the resource with the correct source and params", func() {
				Expect(fakeResource.PutCallCount()).To(Equal(1))

				_, _, putSource, putParams := fakeResource.PutArgsForCall(0)
				Expect(putSource).To(Equal(atc.Source{"some": "super-secret-source"}))
				Expect(putParams).To(Equal(atc.Params{"some-param": "some-value"}))
			})

			It("puts the resource with the io config forwarded", func() {
				Expect(fakeResource.PutCallCount()).To(Equal(1))

				_, ioConfig, _, _ := fakeResource.PutArgsForCall(0)
				Expect(ioConfig.Stdout).To(Equal(stdoutBuf))
				Expect(ioConfig.Stderr).To(Equal(stderrBuf))
			})

			It("runs the get resource action", func() {
				Expect(fakeResource.PutCallCount()).To(Equal(1))
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

			It("stores the version info as the step result", func() {
				Expect(state.StoreResultCallCount()).To(Equal(1))
				sID, sVal := state.StoreResultArgsForCall(0)
				Expect(sID).To(Equal(planID))
				Expect(sVal).To(Equal(exec.VersionInfo{
					Version:  atc.Version{"some": "version"},
					Metadata: []atc.MetadataField{{Name: "some", Value: "metadata"}},
				}))
			})

			Context("when performing the put exits unsuccessfully", func() {
				BeforeEach(func() {
					fakeResource.PutReturns(resource.VersionResult{}, resource.ErrResourceScriptFailed{
						ExitStatus: 42,
					})
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

			Context("when performing the put errors", func() {
				disaster := errors.New("oh no")

				BeforeEach(func() {
					fakeResource.PutReturns(resource.VersionResult{}, disaster)
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

		Context("when find or choosing a worker fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakePool.FindOrChooseWorkerForContainerReturns(nil, disaster)
			})

			It("returns the failure", func() {
				Expect(stepErr).To(Equal(disaster))
			})
		})

		Context("when find or creating a container fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakePool.FindOrChooseWorkerForContainerReturns(fakeWorker, nil)
				fakeWorker.FindOrCreateContainerReturns(nil, disaster)
			})

			It("returns the failure", func() {
				Expect(stepErr).To(Equal(disaster))
			})
		})
	})
})
