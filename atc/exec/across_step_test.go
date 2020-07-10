package exec_test

import (
	"context"

	"code.cloudfoundry.org/lager/lagerctx"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/execfakes"
)

var _ = Describe("AcrossStep", func() {

	var (
		ctx        context.Context
		cancel     func()

		fakeDelegate     *execfakes.FakeBuildStepDelegate

		acrossPlan         atc.AcrossPlan
		state              *execfakes.FakeRunState

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

		fakeDelegate = new(execfakes.FakeBuildStepDelegate)
		fakeDelegate.StderrReturns(stderr)
	})

	AfterEach(func() {
		cancel()
	})

	BeforeEach(func() {
		step := exec.Across(
			acrossPlan,
			nil,
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
})
