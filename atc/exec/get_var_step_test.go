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

	XContext("when the var is in the build vars", func() {
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
			step := exec.NewGetVarStep(
				planID,
				getVarPlan,
				stepMetadata,
				fakeDelegateFactory,
				cache,
			)
			ok, err := step.Run(ctx, state)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())

			sourceName, key, value, redact := state.AddVarArgsForCall(0)
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
		})
	})
})
