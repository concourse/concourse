package exec_test

import (
	"context"
	"errors"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/artifact"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/resource/resourcefakes"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("CheckStep", func() {
	var (
		ctx    context.Context
		cancel func()

		fakeWorker                *workerfakes.FakeWorker
		fakePool                  *workerfakes.FakePool
		fakeStrategy              *workerfakes.FakeContainerPlacementStrategy
		fakeResourceFactory       *resourcefakes.FakeResourceFactory
		fakeResourceConfigFactory *dbfakes.FakeResourceConfigFactory
		fakeSecretManager         *credsfakes.FakeSecrets
		variables                 creds.Variables
		fakeBuild                 *dbfakes.FakeBuild
		fakeDelegate              *execfakes.FakePutDelegate
		checkPlan                 *atc.CheckPlan

		resourceTypes atc.VersionedResourceTypes

		containerMetadata = db.ContainerMetadata{
			WorkingDirectory: resource.ResourcesDir("check"),
			Type:             db.ContainerTypePut,
			StepName:         "some-step",
		}

		stepMetadata testMetadata = []string{"a=1", "b=2"}

		repo  *artifact.Repository
		state *execfakes.FakeRunState

		checkStep *exec.CheckStep
		stepErr   error

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		planID atc.PlanID
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		fakeBuild = new(dbfakes.FakeBuild)
		fakeBuild.IDReturns(42)
		fakeBuild.TeamIDReturns(123)

		planID = atc.PlanID("some-plan-id")

		fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)
		fakePool = new(workerfakes.FakePool)
		fakeWorker = new(workerfakes.FakeWorker)
		fakeResourceFactory = new(resourcefakes.FakeResourceFactory)
		fakeResourceConfigFactory = new(dbfakes.FakeResourceConfigFactory)

		fakeSecretManager = new(credsfakes.FakeSecrets)
		fakeSecretManager.GetReturnsOnCall(0, "super-secret-source", nil, true, nil)
		fakeSecretManager.GetReturnsOnCall(1, "source", nil, true, nil)
		variables = creds.NewVariables(fakeSecretManager, "team", "pipeline")

		fakeDelegate = new(execfakes.FakePutDelegate)
		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
		fakeDelegate.StdoutReturns(stdoutBuf)
		fakeDelegate.StderrReturns(stderrBuf)

		repo = artifact.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactsReturns(repo)

		resourceTypes = atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-resource",
					Type:   "custom-type",
					Source: atc.Source{"some-custom": "((custom-param))"},
				},
				Version: atc.Version{"some-custom": "version"},
			},
		}

		checkPlan = &atc.CheckPlan{
			Name:                   "some-name",
			Resource:               "some-resource",
			Type:                   "some-resource-type",
			Source:                 atc.Source{"some": "((source-param))"},
			Params:                 atc.Params{"some-param": "some-value"},
			Tags:                   []string{"some", "tags"},
			VersionedResourceTypes: resourceTypes,
		}
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		plan := atc.Plan{
			ID:  atc.PlanID(planID),
			Put: checkPlan,
		}

		checkStep = exec.NewCheckStep(
			plan.ID,
			*plan.Put,
			fakeBuild,
			stepMetadata,
			containerMetadata,
			fakeSecretManager,
			fakeResourceFactory,
			fakeResourceConfigFactory,
			fakeStrategy,
			fakePool,
			fakeDelegate,
		)

		stepErr = checkStep.Run(ctx, state)
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
				fakeResource        *resourcefakes.FakeResource
				fakeResourceConfig  *dbfakes.FakeResourceConfig
				fakeVersionedSource *resourcefakes.FakeVersionedSource
			)

			BeforeEach(func() {
				fakeResourceConfig = new(dbfakes.FakeResourceConfig)
				fakeResourceConfig.IDReturns(1)

				fakeResourceConfigFactory.FindOrCreateResourceConfigReturns(fakeResourceConfig, nil)

				fakeVersionedSource = new(resourcefakes.FakeVersionedSource)
				fakeVersionedSource.VersionReturns(atc.Version{"some": "version"})
				fakeVersionedSource.MetadataReturns([]atc.MetadataField{{"some", "metadata"}})

				fakeWorker.NameReturns("some-worker")
				fakePool.FindOrChooseWorkerForContainerReturns(fakeWorker, nil)

				fakeResource = new(resourcefakes.FakeResource)
				fakeResource.PutReturns(fakeVersionedSource, nil)
				fakeResourceFactory.NewResourceForContainerReturns(fakeResource)
			})

			It("finds/chooses a worker and creates a container with the correct type, session, and sources with no inputs specified (meaning it takes all artifacts)", func() {
				Expect(fakePool.FindOrChooseWorkerForContainerCallCount()).To(Equal(1))
				_, actualOwner, actualContainerSpec, actualWorkerSpec, strategy := fakePool.FindOrChooseWorkerForContainerArgsForCall(0)
				Expect(actualOwner).To(Equal(db.NewBuildStepContainerOwner(42, atc.PlanID(planID), 123)))
				Expect(actualContainerSpec.ImageSpec).To(Equal(worker.ImageSpec{
					ResourceType: "some-resource-type",
				}))
				Expect(actualContainerSpec.Tags).To(Equal([]string{"some", "tags"}))
				Expect(actualContainerSpec.TeamID).To(Equal(123))
				Expect(actualContainerSpec.Env).To(Equal([]string{"a=1", "b=2"}))
				Expect(actualContainerSpec.Dir).To(Equal("/tmp/build/check"))
				Expect(actualContainerSpec.Inputs).To(HaveLen(3))
				Expect(actualWorkerSpec).To(Equal(worker.WorkerSpec{
					TeamID:        123,
					Tags:          []string{"some", "tags"},
					ResourceType:  "some-resource-type",
					ResourceTypes: creds.NewVersionedResourceTypes(variables, resourceTypes),
				}))
				Expect(strategy).To(Equal(fakeStrategy))

				_, _, delegate, owner, cm, containerSpec, actualResourceTypes := fakeWorker.FindOrCreateContainerArgsForCall(0)
				Expect(cm).To(Equal(containerMetadata))
				Expect(owner).To(Equal(db.NewBuildStepContainerOwner(42, atc.PlanID(planID), 123)))
				Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
					ResourceType: "some-resource-type",
				}))
				Expect(containerSpec.Tags).To(Equal([]string{"some", "tags"}))
				Expect(containerSpec.TeamID).To(Equal(123))
				Expect(containerSpec.Env).To(Equal([]string{"a=1", "b=2"}))
				Expect(containerSpec.Dir).To(Equal("/tmp/build/check"))
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
				Expect(actualResourceTypes).To(Equal(creds.NewVersionedResourceTypes(variables, resourceTypes)))
				Expect(delegate).To(Equal(fakeDelegate))
			})

			Context("when the inputs are specified", func() {
				BeforeEach(func() {
					checkPlan.Inputs = &atc.InputsConfig{
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

			It("checks the resource with the given context", func() {
				Expect(fakeResource.PutCallCount()).To(Equal(1))
				checkCtx, _, _, _ := fakeResource.PutArgsForCall(0)
				Expect(checkCtx).To(Equal(ctx))
			})

			It("checks the resource with the correct source and params", func() {
				Expect(fakeResource.PutCallCount()).To(Equal(1))

				_, _, checkSource, checkParams := fakeResource.PutArgsForCall(0)
				Expect(checkSource).To(Equal(atc.Source{"some": "super-secret-source"}))
				Expect(checkParams).To(Equal(atc.Params{"some-param": "some-value"}))
			})

			It("checks the resource with the io config forwarded", func() {
				Expect(fakeResource.PutCallCount()).To(Equal(1))

				_, ioConfig, _, _ := fakeResource.PutArgsForCall(0)
				Expect(ioConfig.Stdout).To(Equal(stdoutBuf))
				Expect(ioConfig.Stderr).To(Equal(stderrBuf))
			})

			It("runs the get resource action", func() {
				Expect(fakeResource.PutCallCount()).To(Equal(1))
			})

			It("is successful", func() {
				Expect(checkStep.Succeeded()).To(BeTrue())
			})

			It("saves the build output", func() {
				Expect(fakeBuild.SaveOutputCallCount()).To(Equal(1))

				actualResourceType, actualSource, actualResourceTypes, version, metadata, outputName, resourceName := fakeBuild.SaveOutputArgsForCall(0)
				Expect(actualResourceType).To(Equal("some-resource-type"))
				Expect(actualSource).To(Equal(atc.Source{"some": "super-secret-source"}))
				Expect(actualResourceTypes).To(Equal(creds.NewVersionedResourceTypes(variables, resourceTypes)))
				Expect(version).To(Equal(atc.Version{"some": "version"}))
				Expect(metadata).To(Equal(db.NewResourceConfigMetadataFields([]atc.MetadataField{{"some", "metadata"}})))
				Expect(outputName).To(Equal("some-name"))
				Expect(resourceName).To(Equal("some-resource"))
			})

			Context("when the resource is blank", func() {
				BeforeEach(func() {
					checkPlan.Resource = ""
				})

				It("is successful", func() {
					Expect(checkStep.Succeeded()).To(BeTrue())
				})

				It("does not save the build output", func() {
					Expect(fakeBuild.SaveOutputCallCount()).To(Equal(0))
				})
			})

			It("finishes via the delegate", func() {
				Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
				_, status, info := fakeDelegate.FinishedArgsForCall(0)
				Expect(status).To(Equal(exec.ExitStatus(0)))
				Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
				Expect(info.Metadata).To(Equal([]atc.MetadataField{{"some", "metadata"}}))
			})

			It("stores the version info as the step result", func() {
				Expect(state.StoreResultCallCount()).To(Equal(1))
				sID, sVal := state.StoreResultArgsForCall(0)
				Expect(sID).To(Equal(planID))
				Expect(sVal).To(Equal(exec.VersionInfo{
					Version:  atc.Version{"some": "version"},
					Metadata: []atc.MetadataField{{"some", "metadata"}},
				}))
			})

			Context("when saving the build output fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeBuild.SaveOutputReturns(disaster)
				})

				It("returns the error", func() {
					Expect(stepErr).To(Equal(disaster))
				})
			})

			Context("when performing the check exits unsuccessfully", func() {
				BeforeEach(func() {
					fakeResource.PutReturns(nil, resource.ErrResourceScriptFailed{
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
					Expect(checkStep.Succeeded()).To(BeFalse())
				})
			})

			Context("when performing the check errors", func() {
				disaster := errors.New("oh no")

				BeforeEach(func() {
					fakeResource.PutReturns(nil, disaster)
				})

				It("does not finish the step via the delegate", func() {
					Expect(fakeDelegate.FinishedCallCount()).To(Equal(0))
				})

				It("returns the error", func() {
					Expect(stepErr).To(Equal(disaster))
				})

				It("is not successful", func() {
					Expect(checkStep.Succeeded()).To(BeFalse())
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
