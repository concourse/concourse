package exec_test

import (
	"context"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/vars"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("AcrossStep", func() {
	type vals [3]interface{}

	var (
		ctx    context.Context
		cancel func()

		fakeDelegateFactory *execfakes.FakeBuildStepDelegateFactory
		fakeDelegate        *execfakes.FakeBuildStepDelegate

		step exec.AcrossStep

		acrossVars []atc.AcrossVar
		steps      []exec.ScopedStep
		state      exec.RunState
		failFast   bool

		allVals []vals

		started   chan vals
		terminate map[vals]chan error

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

	stepRun := func(succeeded bool, values vals) func(context.Context, exec.RunState) (bool, error) {
		started := started
		terminate := terminate

		return func(ctx context.Context, state exec.RunState) (bool, error) {
			defer GinkgoRecover()
			for i, v := range acrossVars {
				val, found, _ := state.Get(vars.Reference{Source: ".", Path: v.Var})
				Expect(found).To(BeTrue(), "unset variable "+v.Var)
				Expect(val).To(Equal(values[i]), "invalid value for variable "+v.Var)
			}
			started <- values
			if c, ok := terminate[values]; ok {
				select {
				case err := <-c:
					return false, err
				case <-ctx.Done():
					return false, ctx.Err()
				}
			}
			return succeeded, nil
		}
	}

	scopedStepFactory := func(acrossVars []atc.AcrossVar, values vals) exec.ScopedStep {
		s := new(execfakes.FakeStep)
		s.RunStub = stepRun(true, values)
		return exec.ScopedStep{
			Values: values[:],
			Step:   s,
		}
	}

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		ctx = lagerctx.NewContext(ctx, testLogger)

		state = exec.NewRunState(noopStepper, vars.StaticVariables{}, false)

		stderr = gbytes.NewBuffer()

		fakeDelegate = new(execfakes.FakeBuildStepDelegate)
		fakeDelegate.StderrReturns(stderr)

		fakeDelegateFactory = new(execfakes.FakeBuildStepDelegateFactory)
		fakeDelegateFactory.BuildStepDelegateReturns(fakeDelegate)

		acrossVars = []atc.AcrossVar{
			{
				Var:         "var1",
				Values:      []interface{}{"a1", "a2"},
				MaxInFlight: 2,
			},
			{
				Var:         "var2",
				Values:      []interface{}{"b1", "b2"},
				MaxInFlight: 1,
			},
			{
				Var:         "var3",
				Values:      []interface{}{"c1", "c2"},
				MaxInFlight: 2,
			},
		}
		started = make(chan vals, 8)
		terminate = map[vals]chan error{}

		allVals = []vals{
			{"a1", "b1", "c1"},
			{"a1", "b1", "c2"},

			{"a1", "b2", "c1"},
			{"a1", "b2", "c2"},

			{"a2", "b1", "c1"},
			{"a2", "b1", "c2"},

			{"a2", "b2", "c1"},
			{"a2", "b2", "c2"},
		}

		steps = make([]exec.ScopedStep, len(allVals))
		for i, v := range allVals {
			steps[i] = scopedStepFactory(acrossVars, v)
		}

		failFast = false
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		step = exec.Across(
			acrossVars,
			steps,
			failFast,
			fakeDelegateFactory,
			stepMetadata,
		)
	})

	It("logs a warning to stderr", func() {
		_, err := step.Run(ctx, state)
		Expect(err).ToNot(HaveOccurred())

		Expect(stderr).To(gbytes.Say("WARNING: the across step is experimental"))
		Expect(stderr).To(gbytes.Say("follow RFC #29 for updates"))
	})

	It("initializes the step", func() {
		step.Run(ctx, state)

		Expect(fakeDelegate.InitializingCallCount()).To(Equal(1))
	})

	It("starts the step", func() {
		step.Run(ctx, state)

		Expect(fakeDelegate.StartingCallCount()).To(Equal(1))
	})

	It("finishes the step", func() {
		step.Run(ctx, state)

		Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
	})

	Context("when a var shadows an existing local var", func() {
		BeforeEach(func() {
			state.AddLocalVar("var2", 123, false)
		})

		It("logs a warning to stderr", func() {
			_, err := step.Run(ctx, state)
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr).To(gbytes.Say("WARNING: across step shadows local var 'var2'"))
		})
	})

	Describe("parallel execution", func() {
		BeforeEach(func() {
			for _, v := range allVals {
				terminate[v] = make(chan error, 1)
			}
		})

		It("steps are run in parallel according to the MaxInFlight for each var", func() {
			go step.Run(ctx, state)

			By("running the first stage")
			var receivedVals []vals
			for i := 0; i < 4; i++ {
				receivedVals = append(receivedVals, <-started)
			}
			Expect(receivedVals).To(ConsistOf(
				vals{"a1", "b1", "c1"},
				vals{"a1", "b1", "c2"},
				vals{"a2", "b1", "c1"},
				vals{"a2", "b1", "c2"},
			))
			Consistently(started).ShouldNot(Receive())

			By("the first stage completing successfully")
			for _, v := range receivedVals {
				terminate[v] <- nil
			}

			By("running the second stage")
			receivedVals = []vals{}
			for i := 0; i < 4; i++ {
				receivedVals = append(receivedVals, <-started)
			}
			Expect(receivedVals).To(ConsistOf(
				vals{"a1", "b2", "c1"},
				vals{"a1", "b2", "c2"},
				vals{"a2", "b2", "c1"},
				vals{"a2", "b2", "c2"},
			))
		})

		Context("when fail fast is true", func() {
			BeforeEach(func() {
				failFast = true
			})

			It("stops running steps after a failure", func() {
				By("a step in the first stage failing")
				terminate[allVals[1]] <- nil
				steps[1].Step.(*execfakes.FakeStep).RunStub = stepRun(false, allVals[1])

				By("running the step")
				ok, err := step.Run(ctx, state)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeFalse())

				By("ensuring not all steps were started")
				Expect(started).ToNot(HaveLen(8))
			})
		})

		Context("when fail fast is false", func() {
			BeforeEach(func() {
				failFast = false
			})

			It("allows all steps to run before failing", func() {
				By("a step in the first stage failing")
				steps[1].Step.(*execfakes.FakeStep).RunStub = stepRun(false, allVals[1])

				for _, v := range allVals {
					terminate[v] <- nil
				}

				By("running the step")
				ok, err := step.Run(ctx, state)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeFalse())

				By("ensuring all steps were run")
				Expect(started).To(HaveLen(8))
			})
		})
	})

	Describe("panic recovery", func() {
		Context("when one step panics", func() {
			BeforeEach(func() {
				steps[1].Step.(*execfakes.FakeStep).RunStub = func(context.Context, exec.RunState) (bool, error) {
					panic("something went wrong")
				}
			})

			It("handles it gracefully", func() {
				_, err := step.Run(ctx, state)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("something went wrong"))
			})
		})
	})
})
