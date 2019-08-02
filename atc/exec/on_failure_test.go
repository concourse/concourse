package exec_test

import (
	"context"
	"errors"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/resource/resourcefakes"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/artifact"
	"github.com/concourse/concourse/atc/exec/execfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("On Failure Step", func() {
	var (
		ctx    context.Context
		cancel func()

		step *execfakes.FakeStep
		hook *execfakes.FakeStep

		repo  *artifact.Repository
		state *execfakes.FakeRunState

		onFailureStep exec.Step

		stepErr error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		step = &execfakes.FakeStep{}
		hook = &execfakes.FakeStep{}

		repo = artifact.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactsReturns(repo)

		onFailureStep = exec.OnFailure(step, hook)
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		stepErr = onFailureStep.Run(ctx, state)
	})

	Context("when the step fails", func() {
		BeforeEach(func() {
			step.SucceededReturns(false)
		})

		It("runs the failure hook", func() {
			Expect(step.RunCallCount()).To(Equal(1))
			Expect(hook.RunCallCount()).To(Equal(1))
		})

		It("runs the hook with the run state", func() {
			Expect(hook.RunCallCount()).To(Equal(1))

			_, argsState := hook.RunArgsForCall(0)
			Expect(argsState).To(Equal(state))
		})

		It("propagates the context to the hook", func() {
			runCtx, _ := hook.RunArgsForCall(0)
			Expect(runCtx).To(Equal(ctx))
		})

		It("succeeds", func() {
			Expect(stepErr).ToNot(HaveOccurred())
		})
	})

	Context("when the step errors", func() {
		disaster := errors.New("disaster")

		BeforeEach(func() {
			step.RunReturns(disaster)
		})

		It("does not run the failure hook", func() {
			Expect(step.RunCallCount()).To(Equal(1))
			Expect(hook.RunCallCount()).To(Equal(0))
		})

		It("returns the error", func() {
			Expect(stepErr).To(Equal(disaster))
		})
	})

	Context("when the step succeeds", func() {
		BeforeEach(func() {
			step.SucceededReturns(true)
		})

		It("does not run the failure hook", func() {
			Expect(step.RunCallCount()).To(Equal(1))
			Expect(hook.RunCallCount()).To(Equal(0))
		})

		It("returns nil", func() {
			Expect(stepErr).To(BeNil())
		})
	})

	It("propagates the context to the step", func() {
		runCtx, _ := step.RunArgsForCall(0)
		Expect(runCtx).To(Equal(ctx))
	})

	Describe("Succeeded", func() {
		Context("when step fails and hook fails", func() {
			BeforeEach(func() {
				step.SucceededReturns(false)
				hook.SucceededReturns(false)
			})

			It("returns false", func() {
				Expect(onFailureStep.Succeeded()).To(BeFalse())
			})
		})

		Context("when step fails and hook succeeds", func() {
			BeforeEach(func() {
				step.SucceededReturns(false)
				hook.SucceededReturns(true)
			})

			It("returns false", func() {
				Expect(onFailureStep.Succeeded()).To(BeFalse())
			})
		})

		Context("when step succeeds", func() {
			BeforeEach(func() {
				step.SucceededReturns(true)
			})

			It("returns true", func() {
				Expect(onFailureStep.Succeeded()).To(BeTrue())
			})
		})
	})

	Describe("updateGetStep", func() {
		var getStep exec.Step

		BeforeEach(func() {
			//testLogger := lagertest.NewTestLogger("get-action-test")
			ctx, cancel = context.WithCancel(context.Background())

			//fakeWorker := new(workerfakes.FakeWorker)
			fakeResourceFetcher := new(resourcefakes.FakeFetcher)
			fakePool := new(workerfakes.FakePool)
			fakeStrategy := new(workerfakes.FakeContainerPlacementStrategy)
			fakeResourceCacheFactory := new(dbfakes.FakeResourceCacheFactory)

			fakeSecretManager := new(credsfakes.FakeSecrets)
			fakeSecretManager.GetReturns("super-secret-source", nil, true, nil)

			artifactRepository := artifact.NewRepository()
			state = new(execfakes.FakeRunState)
			state.ArtifactsReturns(artifactRepository)

			fakeVersionedSource := new(resourcefakes.FakeVersionedSource)
			fakeResourceFetcher.FetchReturns(fakeVersionedSource, nil)

			fakeDelegate := new(execfakes.FakeGetDelegate)

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

			containerMetadata := db.ContainerMetadata{
				WorkingDirectory: resource.ResourcesDir("get"),
				PipelineID:       4567,
				Type:             db.ContainerTypeGet,
				StepName:         "some-step",
			}

			stepMetadata := exec.StepMetadata{
				TeamID:       123,
				TeamName:     "some-team",
				BuildID:      42,
				BuildName:    "some-build",
				PipelineID:   4567,
				PipelineName: "some-pipeline",
			}

			getPlan := &atc.GetPlan{
				Name:                   "some-name",
				Type:                   "some-resource-type",
				Source:                 atc.Source{"some": "((source-param))"},
				Params:                 atc.Params{"some-param": "some-value"},
				Tags:                   []string{"some", "tags"},
				Version:                &atc.Version{"some-version": "some-value"},
				VersionedResourceTypes: uninterpolatedResourceTypes,
			}

			plan := atc.Plan{
				ID:  atc.PlanID(67),
				Get: getPlan,
			}

			getStep = exec.NewGetStep(
				plan.ID,
				*plan.Get,
				stepMetadata,
				containerMetadata,
				fakeSecretManager,
				fakeResourceFetcher,
				fakeResourceCacheFactory,
				fakeStrategy,
				fakePool,
				fakeDelegate,
			)


		})

		Context("GetStep", func() {
			BeforeEach(func() {
				onFailureStep = exec.OnFailure(getStep, hook)
				onFailureStep.Run(ctx, state)
			})

			It("Should set registerUponFailure of GetStep", func() {
				Expect(getStep.(*exec.GetStep).GetRegisterUponFailure()).To(BeTrue())
			})
		})

		Context("AggregateStep", func() {
			BeforeEach(func() {
				aggregateStep := exec.AggregateStep{getStep}
				onFailureStep = exec.OnFailure(aggregateStep, hook)
				onFailureStep.Run(ctx, state)
			})

			It("Should set registerUponFailure of GetStep", func() {
				Expect(getStep.(*exec.GetStep).GetRegisterUponFailure()).To(BeTrue())
			})
		})

		Context("InParallelStep", func() {
			BeforeEach(func() {
				inParallelStep := exec.InParallel([]exec.Step{getStep}, 5, true)
				onFailureStep = exec.OnFailure(inParallelStep, hook)
				onFailureStep.Run(ctx, state)
			})

			It("Should set registerUponFailure of GetStep", func() {
				Expect(getStep.(*exec.GetStep).GetRegisterUponFailure()).To(BeTrue())
			})
		})

		Context("LogErrorStep", func() {
			BeforeEach(func() {
				fakeDelegate := new(execfakes.FakeGetDelegate)
				logErrorStep := exec.LogError(getStep, fakeDelegate)
				onFailureStep = exec.OnFailure(logErrorStep, hook)
				onFailureStep.Run(ctx, state)
			})

			It("Should set registerUponFailure of GetStep", func() {
				Expect(getStep.(*exec.GetStep).GetRegisterUponFailure()).To(BeTrue())
			})
		})

		Context("OnSuccessStep-1", func() {
			BeforeEach(func() {
				onSuccessStep := exec.OnSuccess(getStep, hook)
				onFailureStep = exec.OnFailure(onSuccessStep, hook)
				onFailureStep.Run(ctx, state)
			})

			It("Should set registerUponFailure of GetStep", func() {
				Expect(getStep.(*exec.GetStep).GetRegisterUponFailure()).To(BeTrue())
			})
		})

		Context("OnSuccessStep-2", func() {
			BeforeEach(func() {
				onSuccessStep := exec.OnSuccess(hook, getStep)
				onFailureStep = exec.OnFailure(onSuccessStep, hook)
				onFailureStep.Run(ctx, state)
			})

			It("Should set registerUponFailure of GetStep", func() {
				Expect(getStep.(*exec.GetStep).GetRegisterUponFailure()).To(BeTrue())
			})
		})

		Context("TimeoutStep", func() {
			BeforeEach(func() {
				timeoutStep := exec.Timeout(getStep, "5m")
				onFailureStep = exec.OnFailure(timeoutStep, hook)
				onFailureStep.Run(ctx, state)
			})

			It("Should set registerUponFailure of GetStep", func() {
				Expect(getStep.(*exec.GetStep).GetRegisterUponFailure()).To(BeTrue())
			})
		})

		Context("TryStep", func() {
			BeforeEach(func() {
				tryStep := exec.Try(getStep)
				onFailureStep = exec.OnFailure(tryStep, hook)
				onFailureStep.Run(ctx, state)
			})

			It("Should set registerUponFailure of GetStep", func() {
				Expect(getStep.(*exec.GetStep).GetRegisterUponFailure()).To(BeTrue())
			})
		})
	})
})
