package exec_test

import (
	"context"

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

		fakeLockFactory = new(lockfakes.FakeLockFactory)

		cache = gocache.New(0, 0)

		fakeSecrets.GetReturns("some-value", nil, true, nil)

		creds.Register("some-type", fakeManagerFactory)
	})

	AfterEach(func() {
		cancel()
	})

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
	})

	Context("when there is a cache for the var", func() {
		BeforeEach(func() {
			previousState := new(execfakes.FakeRunState)

			step := exec.NewGetVarStep(
				planID,
				getVarPlan,
				stepMetadata,
				fakeDelegateFactory,
				cache,
				fakeLockFactory,
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

	Context("when the same var is accessed by two separate get_var's in the same build", func() {
		// ignore what hppens in the BeforeEach and JustBeforeEach for this test.
		// We don't use the result of any of those vars
		It("one of the two get_var's should wait for the lock to release", func() {
			// run the same step twice at the "same time"

			// fire two instances of get var step
			// instance 1 acquires lock
			// the Get() for instance 1 will block
			// instance 2 attempts to acquire lock and will fail
			// the Get() for instance 1 will unblock right after the other isntance attemps to acquire lock
			// instance 1 will finish
			// instance 2 will acquire lock and finish
		})
	})
})
