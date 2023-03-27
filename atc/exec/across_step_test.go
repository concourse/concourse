package exec_test

import (
	"context"
	"sync/atomic"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/vars"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("AcrossStep", func() {
	type vals [4]interface{}

	var (
		ctx    context.Context
		cancel func()

		fakeDelegateFactory *execfakes.FakeBuildStepDelegateFactory
		fakeDelegate        *execfakes.FakeBuildStepDelegate

		step exec.AcrossStep

		plan  atc.AcrossPlan
		state exec.RunState

		stepperCount        int64
		stepperFailOnCount  int64
		stepperPanicOnCount int64

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

	stepRun := func(succeeded bool) func(context.Context, exec.RunState) (bool, error) {
		started := started
		terminate := terminate

		return func(ctx context.Context, childState exec.RunState) (bool, error) {
			defer GinkgoRecover()

			By("having the correct var values")
			values := vals{}
			for i, v := range plan.Vars {
				val, found, _ := childState.Get(vars.Reference{Source: ".", Path: v.Var})
				Expect(found).To(BeTrue(), "unset variable "+v.Var)
				values[i] = val
			}

			By("running with a child scope")
			Expect(childState.Parent()).To(Equal(state))

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

	stepper := func(plan atc.Plan) exec.Step {
		curCount := atomic.AddInt64(&stepperCount, 1)

		panics := curCount == stepperPanicOnCount

		s := new(execfakes.FakeStep)
		if panics {
			s.RunStub = func(_ context.Context, _ exec.RunState) (bool, error) {
				panic("something went wrong")
			}
		} else {
			successful := curCount != stepperFailOnCount
			s.RunStub = stepRun(successful)
		}
		return s
	}

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		ctx = lagerctx.NewContext(ctx, testLogger)

		state = exec.NewRunState(stepper, vars.StaticVariables{}, false)

		stderr = gbytes.NewBuffer()

		fakeDelegate = new(execfakes.FakeBuildStepDelegate)
		fakeDelegate.StderrReturns(stderr)

		fakeDelegateFactory = new(execfakes.FakeBuildStepDelegateFactory)
		fakeDelegateFactory.BuildStepDelegateReturns(fakeDelegate)

		plan.Vars = []atc.AcrossVar{
			{
				Var:         "var1",
				Values:      []interface{}{"a1", "a2"},
				MaxInFlight: &atc.MaxInFlightConfig{All: true},
			},
			{
				Var:    "var2",
				Values: []interface{}{"b1", "b2"},
			},
			{
				Var:         "var3",
				Values:      []interface{}{"c1", "c2", "c3"},
				MaxInFlight: &atc.MaxInFlightConfig{Limit: 3},
			},
			{
				Var:    "var4",
				Values: []interface{}{"d1", "d2"},
			},
		}
		stepperFailOnCount = -1
		stepperPanicOnCount = -1
		stepperCount = 0

		started = make(chan vals, 24)
		terminate = map[vals]chan error{}

		allVals = []vals{
			{"a1", "b1", "c1", "d1"},
			{"a1", "b1", "c1", "d2"},
			{"a1", "b1", "c2", "d1"},
			{"a1", "b1", "c2", "d2"},
			{"a1", "b1", "c3", "d1"},
			{"a1", "b1", "c3", "d2"},

			{"a1", "b2", "c1", "d1"},
			{"a1", "b2", "c1", "d2"},
			{"a1", "b2", "c2", "d1"},
			{"a1", "b2", "c2", "d2"},
			{"a1", "b2", "c3", "d1"},
			{"a1", "b2", "c3", "d2"},

			{"a2", "b1", "c1", "d1"},
			{"a2", "b1", "c1", "d2"},
			{"a2", "b1", "c2", "d1"},
			{"a2", "b1", "c2", "d2"},
			{"a2", "b1", "c3", "d1"},
			{"a2", "b1", "c3", "d2"},

			{"a2", "b2", "c1", "d1"},
			{"a2", "b2", "c1", "d2"},
			{"a2", "b2", "c2", "d1"},
			{"a2", "b2", "c2", "d2"},
			{"a2", "b2", "c3", "d1"},
			{"a2", "b2", "c3", "d2"},
		}
		plans := make([]atc.VarScopedPlan, len(allVals))
		for i, vals := range allVals {
			// capture the array from the range
			vals := vals
			plans[i] = atc.VarScopedPlan{Values: vals[:]}
		}
		fakeDelegate.ConstructAcrossSubstepsReturns(plans, nil)

		plan.FailFast = false
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		step = exec.Across(
			plan,
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

	It("correctly computes the combinations of var values", func() {
		step.Run(ctx, state)

		Expect(fakeDelegate.ConstructAcrossSubstepsCallCount()).To(Equal(1))
		_, _, valueCombinations := fakeDelegate.ConstructAcrossSubstepsArgsForCall(0)
		Expect(valueCombinations).To(Equal([][]interface{}{
			{"a1", "b1", "c1", "d1"},
			{"a1", "b1", "c1", "d2"},
			{"a1", "b1", "c2", "d1"},
			{"a1", "b1", "c2", "d2"},
			{"a1", "b1", "c3", "d1"},
			{"a1", "b1", "c3", "d2"},

			{"a1", "b2", "c1", "d1"},
			{"a1", "b2", "c1", "d2"},
			{"a1", "b2", "c2", "d1"},
			{"a1", "b2", "c2", "d2"},
			{"a1", "b2", "c3", "d1"},
			{"a1", "b2", "c3", "d2"},

			{"a2", "b1", "c1", "d1"},
			{"a2", "b1", "c1", "d2"},
			{"a2", "b1", "c2", "d1"},
			{"a2", "b1", "c2", "d2"},
			{"a2", "b1", "c3", "d1"},
			{"a2", "b1", "c3", "d2"},

			{"a2", "b2", "c1", "d1"},
			{"a2", "b2", "c1", "d2"},
			{"a2", "b2", "c2", "d1"},
			{"a2", "b2", "c2", "d2"},
			{"a2", "b2", "c3", "d1"},
			{"a2", "b2", "c3", "d2"},
		}))
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
			for i := 0; i < 6; i++ {
				receivedVals = append(receivedVals, <-started)
			}
			Expect(receivedVals).To(ConsistOf(
				vals{"a1", "b1", "c1", "d1"},
				vals{"a1", "b1", "c2", "d1"},
				vals{"a1", "b1", "c3", "d1"},
				vals{"a2", "b1", "c1", "d1"},
				vals{"a2", "b1", "c2", "d1"},
				vals{"a2", "b1", "c3", "d1"},
			))
			Consistently(started).ShouldNot(Receive())

			By("the first stage completing successfully")
			for _, v := range receivedVals {
				terminate[v] <- nil
			}

			By("running the second stage")
			receivedVals = []vals{}
			for i := 0; i < 6; i++ {
				receivedVals = append(receivedVals, <-started)
			}
			Expect(receivedVals).To(ConsistOf(
				vals{"a1", "b1", "c1", "d2"},
				vals{"a1", "b1", "c2", "d2"},
				vals{"a1", "b1", "c3", "d2"},
				vals{"a2", "b1", "c1", "d2"},
				vals{"a2", "b1", "c2", "d2"},
				vals{"a2", "b1", "c3", "d2"},
			))
			Consistently(started).ShouldNot(Receive())

			By("the second stage completing successfully")
			for _, v := range receivedVals {
				terminate[v] <- nil
			}

			By("running the third stage")
			receivedVals = []vals{}
			for i := 0; i < 6; i++ {
				receivedVals = append(receivedVals, <-started)
			}
			Expect(receivedVals).To(ConsistOf(
				vals{"a1", "b2", "c1", "d1"},
				vals{"a1", "b2", "c2", "d1"},
				vals{"a1", "b2", "c3", "d1"},
				vals{"a2", "b2", "c1", "d1"},
				vals{"a2", "b2", "c2", "d1"},
				vals{"a2", "b2", "c3", "d1"},
			))
			Consistently(started).ShouldNot(Receive())

			By("the third stage completing successfully")
			for _, v := range receivedVals {
				terminate[v] <- nil
			}

			By("running the forth stage")
			receivedVals = []vals{}
			for i := 0; i < 6; i++ {
				receivedVals = append(receivedVals, <-started)
			}
			Expect(receivedVals).To(ConsistOf(
				vals{"a1", "b2", "c1", "d2"},
				vals{"a1", "b2", "c2", "d2"},
				vals{"a1", "b2", "c3", "d2"},
				vals{"a2", "b2", "c1", "d2"},
				vals{"a2", "b2", "c2", "d2"},
				vals{"a2", "b2", "c3", "d2"},
			))
			Consistently(started).ShouldNot(Receive())

			By("the forth stage completing successfully")
			for _, v := range receivedVals {
				terminate[v] <- nil
			}
		})

		Context("when fail fast is true", func() {
			BeforeEach(func() {
				plan.FailFast = true

				stepperFailOnCount = 2
			})

			It("stops running steps after a failure", func() {
				// Allow the failed step to terminate
				terminate[allVals[0]] <- nil

				By("running the step")
				ok, err := step.Run(ctx, state)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeFalse())

				By("ensuring not all steps were started")
				Expect(started).ToNot(HaveLen(24))
			})
		})

		Context("when fail fast is false", func() {
			BeforeEach(func() {
				plan.FailFast = false

				stepperFailOnCount = 2
			})

			It("allows all steps to run before failing", func() {
				for _, v := range allVals {
					terminate[v] <- nil
				}

				By("running the step")
				ok, err := step.Run(ctx, state)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeFalse())

				By("ensuring all steps were run")
				Expect(started).To(HaveLen(24))
			})
		})
	})

	Describe("panic recovery", func() {
		Context("when one step panics", func() {
			BeforeEach(func() {
				stepperPanicOnCount = 2
			})

			It("handles it gracefully", func() {
				_, err := step.Run(ctx, state)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("something went wrong"))
			})
		})
	})
})
