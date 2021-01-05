package exec_test

import (
	"context"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	gocache "github.com/patrickmn/go-cache"
	"go.opentelemetry.io/otel/api/trace"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/execfakes"
)

var _ = Describe("GetVarStep", func() {
	var (
		ctx        context.Context
		cancel     func()
		testLogger *lagertest.TestLogger

		fakeDelegate        *execfakes.FakeBuildStepDelegate
		fakeDelegateFactory *execfakes.FakeBuildStepDelegateFactory

		spanCtx context.Context

		getVarPlan atc.GetVarPlan
		state      *execfakes.FakeRunState

		fakeManagerFactory *credsfakes.FakeManagerFactory
		fakeManager        *credsfakes.FakeManager
		fakeSecretsFactory *credsfakes.FakeSecretsFactory
		fakeSecrets        *credsfakes.FakeSecrets
		fakeLockFactory    *lockfakes.FakeLockFactory
		fakeLock           *lockfakes.FakeLock

		cache *gocache.Cache

		step    exec.Step
		stepOk  bool
		stepErr error

		stepMetadata = exec.StepMetadata{
			TeamID:       123,
			TeamName:     "some-team",
			BuildID:      42,
			BuildName:    "some-build",
			PipelineID:   4567,
			PipelineName: "some-pipeline",
		}

		stdout, stderr *gbytes.Buffer

		planID atc.PlanID = "56"
	)

	BeforeEach(func() {
		testLogger = lagertest.NewTestLogger("var-step-test")
		ctx, cancel = context.WithCancel(context.Background())
		ctx = lagerctx.NewContext(ctx, testLogger)

		state = new(execfakes.FakeRunState)

		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()

		fakeDelegate = new(execfakes.FakeBuildStepDelegate)
		fakeDelegate.StdoutReturns(stdout)
		fakeDelegate.StderrReturns(stderr)

		spanCtx = context.Background()
		fakeDelegate.StartSpanReturns(spanCtx, trace.NoopSpan{})

		fakeDelegateFactory = new(execfakes.FakeBuildStepDelegateFactory)
		fakeDelegateFactory.BuildStepDelegateReturns(fakeDelegate)

		getVarPlan = atc.GetVarPlan{
			Name: "some-source-name",
			Path: "some-var",
			Type: "some-type",
			Source: atc.Source{
				"some": "source",
			},
		}

		fakeManagerFactory = new(credsfakes.FakeManagerFactory)
		fakeManager = new(credsfakes.FakeManager)
		fakeManagerFactory.NewInstanceReturns(fakeManager, nil)

		fakeSecretsFactory = new(credsfakes.FakeSecretsFactory)
		fakeSecrets = new(credsfakes.FakeSecrets)
		fakeSecretsFactory.NewSecretsReturns(fakeSecrets)
		fakeManager.NewSecretsFactoryReturns(fakeSecretsFactory, nil)

		fakeLock = new(lockfakes.FakeLock)
		fakeLockFactory = new(lockfakes.FakeLockFactory)
		fakeLockFactory.AcquireReturns(fakeLock, true, nil)

		cache = gocache.New(0, 0)

		fakeSecrets.GetReturns("some-value", nil, true, nil)

		creds.Register("some-type", fakeManagerFactory)
	})

	AfterEach(func() {
		cancel()
	})

	Context("Single get_var step", func() {
		JustBeforeEach(func() {
			step = exec.NewGetVarStep(
				planID,
				getVarPlan,
				stepMetadata,
				fakeDelegateFactory,
				cache,
				fakeLockFactory,
			)

			stepOk, stepErr = step.Run(ctx, state)
		})

		It("gets the var and stores it as the step result", func() {
			Expect(stepOk).To(BeTrue())
			Expect(stepErr).ToNot(HaveOccurred())

			Expect(fakeManagerFactory.NewInstanceCallCount()).To(Equal(1))
			config := fakeManagerFactory.NewInstanceArgsForCall(0)
			Expect(config).To(Equal(getVarPlan.Source))

			Expect(fakeManager.InitCallCount()).To(Equal(1))

			Expect(fakeSecrets.GetCallCount()).To(Equal(1))
			path := fakeSecrets.GetArgsForCall(0)
			Expect(path).To(Equal("some-var"))

			Expect(state.AddVarCallCount()).To(Equal(1))
			varName, varPath, varValue, redact := state.AddVarArgsForCall(0)
			Expect(varName).To(Equal("some-source-name"))
			Expect(varPath).To(Equal("some-var"))
			Expect(varValue).To(Equal("some-value"))
			Expect(redact).To(BeTrue())

			Expect(fakeManager.CloseCallCount()).To(Equal(1))
			Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
		})

		Context("when the var does not exist", func() {
			BeforeEach(func() {
				fakeSecrets.GetReturns("", nil, false, nil)
			})

			It("fails with var not found error", func() {
				Expect(stepOk).To(BeFalse())
				Expect(stepErr).To(HaveOccurred())
				Expect(stepErr).To(Equal(exec.VarNotFoundError{Name: "some-var"}))
			})

			It("releases the lock", func() {
				Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
			})
		})

		Context("when the var is in the build vars", func() {
			BeforeEach(func() {
				state.GetReturns(nil, true, nil)
			})

			It("uses the stored var and does not refetch", func() {
				Expect(stepOk).To(BeTrue())
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(fakeSecrets.GetCallCount()).To(Equal(0))
				Expect(state.AddVarCallCount()).To(Equal(0))
			})

			It("releases the lock", func() {
				Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
			})
		})

		Context("when there is a cache for the var", func() {
			BeforeEach(func() {
				previousState := new(execfakes.FakeRunState)
				tempfakeLock := new(lockfakes.FakeLock)
				tempfakeLockFactory := new(lockfakes.FakeLockFactory)
				tempfakeLockFactory.AcquireReturns(tempfakeLock, true, nil)

				step := exec.NewGetVarStep(
					planID,
					getVarPlan,
					stepMetadata,
					fakeDelegateFactory,
					cache,
					tempfakeLockFactory,
				)

				ok, err := step.Run(ctx, previousState)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())

				Expect(fakeSecrets.GetCallCount()).To(Equal(1))
				Expect(previousState.AddVarCallCount()).To(Equal(1))

				sourceName, key, value, redact := previousState.AddVarArgsForCall(0)
				Expect(sourceName).To(Equal("some-source-name"))
				Expect(key).To(Equal("some-var"))
				Expect(value).To(Equal("some-value"))
				Expect(redact).To(BeTrue())
			})

			It("uses the cached var and does not refetch", func() {
				Expect(stepOk).To(BeTrue())
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(fakeSecrets.GetCallCount()).To(Equal(1))
				Expect(state.AddVarCallCount()).To(Equal(1))

				sourceName, key, value, redact := state.AddVarArgsForCall(0)
				Expect(sourceName).To(Equal("some-source-name"))
				Expect(key).To(Equal("some-var"))
				Expect(value).To(Equal("some-value"))
				Expect(redact).To(BeTrue())
			})

			It("releases the lock", func() {
				Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
			})
		})

		Context("when reveal is true", func() {
			BeforeEach(func() {
				getVarPlan.Reveal = true
			})
			It("the var is not redactable", func() {
				Expect(stepOk).To(BeTrue())
				Expect(stepErr).ToNot(HaveOccurred())

				_, _, _, redact := state.AddVarArgsForCall(0)
				Expect(redact).To(BeFalse())
			})
		})

	})

	Context("Multiple get_var steps", func() {
		Context("when the same var is accessed by two separate get_var's in the same build", func() {
			var (
				acquired bool
				fakeLock *lockfakes.FakeLock
				step1    exec.Step
				step2    exec.Step
			)

			BeforeEach(func() {
				acquired = false

				fakeLock = new(lockfakes.FakeLock)
				fakeLock.ReleaseStub = func() error { return nil }

				fakeLockFactory.AcquireStub = func(lager.Logger, lock.LockID) (lock.Lock, bool, error) {
					if !acquired {
						acquired = true
						return fakeLock, true, nil
					} else {
						return nil, false, nil
					}
				}

				step1 = exec.NewGetVarStep(
					"1",
					getVarPlan,
					stepMetadata,
					fakeDelegateFactory,
					cache,
					fakeLockFactory,
				)

				step2 = exec.NewGetVarStep(
					"2",
					getVarPlan,
					stepMetadata,
					fakeDelegateFactory,
					cache,
					fakeLockFactory,
				)
			})

			It("one of the two get_var's should wait for the lock to release", func() {
				step1Ok, step1Err := step1.Run(ctx, state)
				Expect(step1Ok).To(BeTrue())
				Expect(step1Err).ToNot(HaveOccurred())

				go func() {
					step2Ok, step2Err := step2.Run(ctx, state)
					Expect(step2Ok).To(BeTrue())
					Expect(step2Err).ToNot(HaveOccurred())
				}()

				Expect(fakeLockFactory.AcquireCallCount()).To(Equal(1))
				Expect(state.GetCallCount()).To(Equal(1))
				Expect(fakeSecrets.GetCallCount()).To(Equal(1))

				By("releasing the lock")
				acquired = false

				Eventually(fakeLockFactory.AcquireCallCount).Should(Equal(2))
				Expect(state.GetCallCount()).To(Equal(2))

				By("does not fetch the var twice")
				Expect(fakeSecrets.GetCallCount()).To(Equal(1))
			})
		})
	})
})
