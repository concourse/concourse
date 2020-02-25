package exec_test

import (
	"context"
	"errors"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/resource/resourcefakes"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/concourse/concourse/vars/varsfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CheckStep", func() {

	var (
		fakeRunState        *execfakes.FakeRunState
		fakeResourceFactory *resourcefakes.FakeResourceFactory
		fakePool            *workerfakes.FakePool
		fakeStrategy        *workerfakes.FakeContainerPlacementStrategy
		fakeDelegate        *execfakes.FakeCheckDelegate
		fakeClient          *workerfakes.FakeClient

		checkStep *exec.CheckStep
		checkPlan atc.CheckPlan

		err error
	)

	BeforeEach(func() {
		fakeRunState = new(execfakes.FakeRunState)
		fakeResourceFactory = new(resourcefakes.FakeResourceFactory)
		fakePool = new(workerfakes.FakePool)
		fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)
		fakeDelegate = new(execfakes.FakeCheckDelegate)
		fakeClient = new(workerfakes.FakeClient)
	})

	JustBeforeEach(func() {
		containerMetadata := db.ContainerMetadata{}
		stepMetadata := exec.StepMetadata{}
		planID := atc.PlanID("some-plan-id")

		checkStep = exec.NewCheckStep(
			planID,
			checkPlan,
			stepMetadata,
			fakeResourceFactory,
			containerMetadata,
			fakeStrategy,
			fakePool,
			fakeDelegate,
			fakeClient,
		)

		err = checkStep.Run(context.Background(), fakeRunState)
	})

	Context("having credentials in the config", func() {
		BeforeEach(func() {
			checkPlan = atc.CheckPlan{
				Source: atc.Source{"some": "((super-secret-source))"},
			}
		})

		Context("having cred evaluation failing", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = errors.New("creds-err")

				fakeCredVarsTracker := new(varsfakes.FakeCredVarsTracker)
				fakeCredVarsTracker.GetReturns(nil, false, expectedErr)

				fakeDelegate.VariablesReturns(fakeCredVarsTracker)
			})

			It("errors", func() {
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, expectedErr)).To(BeTrue())
			})
		})
	})

	Context("having credentials in a resource type", func() {
		BeforeEach(func() {
			resTypes := atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Source: atc.Source{
							"some-custom": "((super-secret-source))",
						},
					},
				},
			}

			checkPlan = atc.CheckPlan{
				Source:                 atc.Source{"some": "super-secret-source"},
				VersionedResourceTypes: resTypes,
			}
		})

		Context("having cred evaluation failing", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = errors.New("creds-err")

				fakeCredVarsTracker := new(varsfakes.FakeCredVarsTracker)
				fakeCredVarsTracker.GetReturns(nil, false, expectedErr)

				fakeDelegate.VariablesReturns(fakeCredVarsTracker)
			})

			It("errors", func() {
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, expectedErr)).To(BeTrue())
			})
		})
	})

	Context("having a timeout that fails parsing", func() {
		BeforeEach(func() {
			checkPlan = atc.CheckPlan{
				Timeout: "th1s_15_n07_r1gh7",
			}
		})

		It("errors", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid duration"))
		})
	})

	Context("with a resonable configuration", func() {
		BeforeEach(func() {
			checkPlan = atc.CheckPlan{
				Timeout: "10s",
			}
		})

		Context("having RunCheckStep erroring", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = errors.New("run-check-step-err")

				fakeClient.RunCheckStepReturns(nil, expectedErr)
			})

			It("errors", func() {
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, expectedErr)).To(BeTrue())
			})
		})

		Context("having SaveVersions failing", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = errors.New("save-versions-err")

				fakeDelegate.SaveVersionsReturns(expectedErr)
			})

			It("errors", func() {
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, expectedErr)).To(BeTrue())
			})
		})
	})

})
