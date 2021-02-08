package exec_test

import (
	"context"
	"errors"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/vars"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RunState", func() {
	var (
		stepper     exec.Stepper
		steppedPlan atc.Plan
		fakeStep    *execfakes.FakeStep

		varSources atc.VarSourceConfigs

		state exec.RunState
	)

	BeforeEach(func() {
		fakeStep = new(execfakes.FakeStep)
		stepper = func(plan atc.Plan) exec.Step {
			steppedPlan = plan
			return fakeStep
		}

		varSources = atc.VarSourceConfigs{
			{
				Name:   "some-var-source",
				Type:   "registry",
				Config: map[string]string{"some": "config"},
			},
		}

		state = exec.NewRunState(stepper, varSources, false)
	})

	Describe("Run", func() {
		var ctx context.Context
		var plan atc.Plan

		var runOk bool
		var runErr error

		BeforeEach(func() {
			ctx = context.Background()
			plan = atc.Plan{
				ID: "some-plan",
				LoadVar: &atc.LoadVarPlan{
					Name: "foo",
					File: "bar",
				},
			}

			fakeStep.RunReturns(true, nil)
		})

		JustBeforeEach(func() {
			runOk, runErr = state.Run(ctx, plan)
		})

		It("constructs and runs a step for the plan", func() {
			Expect(steppedPlan).To(Equal(plan))
			Expect(fakeStep.RunCallCount()).To(Equal(1))
			runCtx, runState := fakeStep.RunArgsForCall(0)
			Expect(runCtx).To(Equal(ctx))
			Expect(runState).To(Equal(state))
		})

		Context("when the step succeeds", func() {
			BeforeEach(func() {
				fakeStep.RunReturns(true, nil)
			})

			It("succeeds", func() {
				Expect(runOk).To(BeTrue())
			})
		})

		Context("when the step fails", func() {
			BeforeEach(func() {
				fakeStep.RunReturns(false, nil)
			})

			It("fails", func() {
				Expect(runOk).To(BeFalse())
			})
		})

		Context("when the step errors", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeStep.RunReturns(false, disaster)
			})

			It("returns the error", func() {
				Expect(runErr).To(Equal(disaster))
			})
		})
	})

	Describe("Result", func() {
		var to int

		BeforeEach(func() {
			to = 42
		})

		Context("when no result is present", func() {
			It("returns false", func() {
				Expect(state.Result("some-id", &to)).To(BeFalse())
			})

			It("does not mutate the var", func() {
				state.Result("some-id", &to)
				Expect(to).To(Equal(42))
			})
		})

		Context("when a result under a different id is present", func() {
			BeforeEach(func() {
				state.StoreResult("other", 43)
			})

			It("returns false", func() {
				Expect(state.Result("some-id", &to)).To(BeFalse())
			})

			It("does not mutate the var", func() {
				state.Result("some-id", &to)
				Expect(to).To(Equal(42))
			})
		})

		Context("when a result under the given id is present", func() {
			BeforeEach(func() {
				state.StoreResult("some-id", 123)
			})

			It("returns true", func() {
				Expect(state.Result("some-id", &to)).To(BeTrue())
			})

			It("mutates the var", func() {
				state.Result("some-id", &to)
				Expect(to).To(Equal(123))
			})

			Context("but with a different type", func() {
				BeforeEach(func() {
					state.StoreResult("some-id", "one hundred and twenty-three")
				})

				It("returns false", func() {
					Expect(state.Result("some-id", &to)).To(BeFalse())
				})

				It("does not mutate the var", func() {
					state.Result("some-id", &to)
					Expect(to).To(Equal(42))
				})
			})
		})

		Context("when the argument to set the result onto is an interface type", func() {
			var to interface{}
			BeforeEach(func() {
				state.StoreResult("some-id", "some-result")
			})

			It("populates the result", func() {
				state.Result("some-id", &to)
				Expect(to).To(Equal("some-result"))
			})
		})
	})

	Describe("NewScope", func() {
		It("maintains a reference to the parent", func() {
			Expect(state.NewScope().Parent()).To(Equal(state))
		})

		It("can access vars from parent scope", func() {
			state.LocalVariables().SetVar(".", "hello", "world", false)
			scope := state.NewScope()
			val, _, _ := scope.LocalVariables().Get(vars.Reference{Source: ".", Path: "hello"})
			Expect(val).To(Equal("world"))
		})

		It("adding vars does not affect the vars from the parent scope", func() {
			scope := state.NewScope()
			scope.LocalVariables().SetVar(".", "hello", "world", false)
			_, found, _ := state.LocalVariables().Get(vars.Reference{Source: ".", Path: "hello"})
			Expect(found).To(BeFalse())
		})

		It("current scope is preferred over parent scope", func() {
			state.LocalVariables().SetVar(".", "a", 1, false)
			scope := state.NewScope()
			scope.LocalVariables().SetVar(".", "a", 2, false)

			val, _, _ := scope.LocalVariables().Get(vars.Reference{Source: ".", Path: "a"})
			Expect(val).To(Equal(2))
		})

		It("results set in parent scope are accessible in child", func() {
			parent := state
			child := parent.NewScope()

			parent.StoreResult("id", "hello")

			var dst string
			child.Result("id", &dst)
			Expect(dst).To(Equal("hello"))
		})

		It("results set in child scope are accessible in parent", func() {
			parent := state
			child := parent.NewScope()

			child.StoreResult("id", "hello")

			var dst string
			state.Result("id", &dst)
			Expect(dst).To(Equal("hello"))
		})

		It("has an artifact scope inheriting from the outer scope", func() {
			Expect(state.NewScope().ArtifactRepository().Parent()).To(Equal(state.ArtifactRepository()))
		})

		Describe("TrackedVarsMap", func() {
			BeforeEach(func() {
				state = exec.NewRunState(stepper, varSources, true)
			})

			It("can track on both scopes", func() {
				state.LocalVariables().SetVar(".", "a", "parent", true)
				scope := state.NewScope()
				scope.LocalVariables().SetVar(".", "a", "child", true)

				mapit := vars.TrackedVarsMap{}
				state.IterateInterpolatedCreds(mapit)
				Expect(mapit[".:a"]).To(Equal("parent"))

				mapit = vars.TrackedVarsMap{}
				scope.IterateInterpolatedCreds(mapit)
				Expect(mapit[".:a"]).To(Equal("child"))
			})

			It("prefers the value set in the current scope over the parent scope", func() {
				state.LocalVariables().SetVar(".", "a", "from parent", true)
				scope := state.NewScope()
				scope.LocalVariables().SetVar(".", "a", "from child", true)

				mapit := vars.TrackedVarsMap{}
				scope.IterateInterpolatedCreds(mapit)

				Expect(mapit[".:a"]).To(Equal("from child"))
			})
		})
	})

	Describe("IterateInterpolatedCreds", func() {
		Context("when vars are tracked from local vars and using the Track method", func() {
			BeforeEach(func() {
				state = exec.NewRunState(stepper, varSources, true)

				state.LocalVariables().SetVar(".", "a", "value", true)
				state.Track(vars.Reference{
					Source: "var-source",
					Path:   "foo",
				}, "some-value")
				state.Track(vars.Reference{
					Source: "other-source",
					Path:   "bar",
					Fields: []string{"some-field"},
				}, "some-field-value")
			})

			It("tracks all vars", func() {
				mapit := vars.TrackedVarsMap{}
				state.IterateInterpolatedCreds(mapit)
				Expect(mapit).To(HaveLen(3))
				Expect(mapit[".:a"]).To(Equal("value"))
				Expect(mapit["var-source:foo"]).To(Equal("some-value"))
				Expect(mapit["other-source:bar.some-field"]).To(Equal("some-field-value"))
			})
		})
	})
})
