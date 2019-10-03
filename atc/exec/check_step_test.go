package exec_test

import (
	"context"
	"errors"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/resource/resourcefakes"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CheckStep", func() {
	var (
		ctx    context.Context
		cancel func()

		fakeWorker          *workerfakes.FakeWorker
		fakePool            *workerfakes.FakePool
		fakeStrategy        *workerfakes.FakeContainerPlacementStrategy
		fakeResourceFactory *resourcefakes.FakeResourceFactory
		fakeSecretManager   *credsfakes.FakeSecrets
		fakeDelegate        *execfakes.FakeCheckDelegate
		checkPlan           *atc.CheckPlan

		interpolatedResourceTypes atc.VersionedResourceTypes

		containerMetadata = db.ContainerMetadata{
			Type:     db.ContainerTypeCheck,
			StepName: "some-step",
		}

		stepMetadata = exec.StepMetadata{
			ResourceConfigID:   1,
			BaseResourceTypeID: 1,
			TeamID:             123,
		}

		owner = db.NewResourceConfigCheckSessionContainerOwner(stepMetadata.ResourceConfigID, stepMetadata.BaseResourceTypeID, db.ContainerOwnerExpiries{
			Min: 5 * time.Minute,
			Max: 1 * time.Hour,
		})

		repo  *artifact.Repository
		state *execfakes.FakeRunState

		checkStep *exec.CheckStep
		stepErr   error

		planID atc.PlanID
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		planID = atc.PlanID("some-plan-id")

		fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)
		fakePool = new(workerfakes.FakePool)
		fakeWorker = new(workerfakes.FakeWorker)
		fakeResourceFactory = new(resourcefakes.FakeResourceFactory)

		fakeSecretManager = new(credsfakes.FakeSecrets)
		fakeSecretManager.GetReturnsOnCall(0, "super-secret-source", nil, true, nil)
		fakeSecretManager.GetReturnsOnCall(1, "source", nil, true, nil)

		fakeDelegate = new(execfakes.FakeCheckDelegate)

		repo = artifact.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactsReturns(repo)

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

		checkPlan = &atc.CheckPlan{
			Name:                   "some-name",
			Type:                   "some-resource-type",
			Source:                 atc.Source{"some": "super-secret-source"},
			Tags:                   []string{"some", "tags"},
			Timeout:                "10s",
			FromVersion:            atc.Version{"some-custom": "version"},
			VersionedResourceTypes: interpolatedResourceTypes,
		}
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		plan := atc.Plan{
			ID:    atc.PlanID(planID),
			Check: checkPlan,
		}

		checkStep = exec.NewCheckStep(
			plan.ID,
			*plan.Check,
			stepMetadata,
			containerMetadata,
			fakeResourceFactory,
			fakeStrategy,
			fakePool,
			fakeDelegate,
		)

		stepErr = checkStep.Run(ctx, state)
	})

	Context("when find or choosing worker succeeds", func() {
		var (
			fakeResource *resourcefakes.FakeResource
			versions     []atc.Version
		)

		BeforeEach(func() {
			fakeWorker.NameReturns("some-worker")
			fakePool.FindOrChooseWorkerForContainerReturns(fakeWorker, nil)

			fakeResource = new(resourcefakes.FakeResource)
			fakeResourceFactory.NewResourceForContainerReturns(fakeResource)
		})

		It("finds or chooses a worker", func() {
			Expect(fakePool.FindOrChooseWorkerForContainerCallCount()).To(Equal(1))
			_, _, actualOwner, actualContainerSpec, actualWorkerSpec, strategy := fakePool.FindOrChooseWorkerForContainerArgsForCall(0)

			Expect(actualOwner).To(Equal(owner))

			Expect(actualContainerSpec.ImageSpec).To(Equal(worker.ImageSpec{
				ResourceType: "some-resource-type",
			}))
			Expect(actualContainerSpec.Tags).To(Equal([]string{"some", "tags"}))
			Expect(actualContainerSpec.TeamID).To(Equal(123))
			Expect(actualContainerSpec.Env).To(Equal(stepMetadata.Env()))

			Expect(actualWorkerSpec).To(Equal(worker.WorkerSpec{
				ResourceType:  "some-resource-type",
				Tags:          atc.Tags{"some", "tags"},
				TeamID:        stepMetadata.TeamID,
				ResourceTypes: interpolatedResourceTypes,
			}))

			Expect(strategy).To(Equal(fakeStrategy))
		})

		It("creates a container with the correct type and owner", func() {
			_, _, delegate, actualOwner, actualContainerMetadata, actualContainerSpec, actualResourceTypes := fakeWorker.FindOrCreateContainerArgsForCall(0)

			Expect(actualOwner).To(Equal(owner))

			Expect(actualContainerSpec.ImageSpec).To(Equal(worker.ImageSpec{
				ResourceType: "some-resource-type",
			}))
			Expect(actualContainerMetadata).To(Equal(containerMetadata))
			Expect(actualContainerSpec.Tags).To(Equal([]string{"some", "tags"}))
			Expect(actualContainerSpec.TeamID).To(Equal(123))
			Expect(actualContainerSpec.Env).To(Equal(stepMetadata.Env()))

			Expect(actualResourceTypes).To(Equal(interpolatedResourceTypes))
			Expect(delegate).To(Equal(fakeDelegate))
		})

		Context("when the timeout cannot be parsed", func() {
			BeforeEach(func() {
				checkPlan.Timeout = "bad-value"
			})

			It("fails to parse the timeout and returns the error", func() {
				Expect(stepErr).To(HaveOccurred())
				Expect(stepErr).To(MatchError("time: invalid duration bad-value"))
			})
		})

		It("times out after the specified timeout", func() {
			now := time.Now()
			ctx, _, _ := fakeResource.CheckArgsForCall(0)
			deadline, _ := ctx.Deadline()
			Expect(deadline).Should(BeTemporally("~", now.Add(10*time.Second), time.Second))
		})

		It("runs the check resource action", func() {
			Expect(fakeResource.CheckCallCount()).To(Equal(1))
		})

		Context("when resource check succeeds", func() {
			BeforeEach(func() {
				fakeResource.CheckReturns(versions, nil)
			})

			It("saves the versions", func() {
				Expect(fakeDelegate.SaveVersionsCallCount()).To(Equal(1))

				actualVersions := fakeDelegate.SaveVersionsArgsForCall(0)
				Expect(actualVersions).To(Equal(versions))
			})
		})

		Context("when performing the check fails", func() {
			BeforeEach(func() {
				fakeResource.CheckReturns(nil, errors.New("nope"))
			})

			It("returns error", func() {
				Expect(stepErr).To(HaveOccurred())
			})

			It("is not successful", func() {
				Expect(checkStep.Succeeded()).To(BeFalse())
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
