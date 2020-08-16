package exec_test

import (
	"context"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/vars"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/execfakes"
)

var _ = Describe("AcrossStep", func() {

	var (
		ctx    context.Context
		cancel func()

		fakeDelegate *execfakes.FakeBuildStepDelegate

		buildVars *vars.BuildVariables

		step     exec.InParallelStep
		varNames []string
		state    *execfakes.FakeRunState

		stepMetadata = exec.StepMetadata{
			TeamID:       123,
			TeamName:     "some-team",
			BuildID:      42,
			BuildName:    "some-build",
			PipelineID:   4567,
			PipelineName: "some-pipeline",
		}

		stderr *gbytes.Buffer
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		ctx = lagerctx.NewContext(ctx, testLogger)

		state = new(execfakes.FakeRunState)

		stderr = gbytes.NewBuffer()

		buildVars = vars.NewBuildVariables(vars.StaticVariables{}, false)

		fakeDelegate = new(execfakes.FakeBuildStepDelegate)
		fakeDelegate.StderrReturns(stderr)

		varNames = []string{"var1", "var2"}
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		fakeDelegate.VariablesReturns(buildVars.NewLocalScope())

		step := exec.Across(
			step,
			varNames,
			fakeDelegate,
			stepMetadata,
		)

		stepErr := step.Run(ctx, state)
		Expect(stepErr).ToNot(HaveOccurred())
	})

	It("logs a warning to stderr", func() {
		Expect(stderr).To(gbytes.Say("WARNING: the across step is experimental"))
		Expect(stderr).To(gbytes.Say("follow RFC #29 for updates"))
	})

	It("initializes the step", func() {
		Expect(fakeDelegate.InitializingCallCount()).To(Equal(1))
	})

	It("starts the step", func() {
		Expect(fakeDelegate.StartingCallCount()).To(Equal(1))
	})

	It("finishes the step", func() {
		Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
	})

	Context("when a var shadows an existing local var", func() {
		BeforeEach(func() {
			buildVars.AddLocalVar("var2", 123, false)
		})

		It("logs a warning to stderr", func() {
			Expect(stderr).To(gbytes.Say("WARNING: across step shadows local var 'var2'"))
		})
	})
})
