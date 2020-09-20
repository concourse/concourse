package exec_test

import (
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/vars"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RunState", func() {
	var (
		state    exec.RunState
		credVars vars.Variables
	)

	BeforeEach(func() {
		credVars = vars.StaticVariables{"k1": "v1", "k2": "v2", "k3": "v3"}
		state = exec.NewRunState(credVars, false)
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
	})

	Describe("Get", func() {
		BeforeEach(func() {
			state = exec.NewRunState(credVars, false)
		})

		It("fetches from cred vars", func() {
			val, found, err := state.Get(vars.VariableDefinition{Ref: vars.VariableReference{Path: "k1"}})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(val).To(Equal("v1"))
		})

		Context("when redaction is enabled", func() {
			BeforeEach(func() {
				state = exec.NewRunState(credVars, true)
			})

			It("fetched variables are tracked", func() {
				state.Get(vars.VariableDefinition{Ref: vars.VariableReference{Path: "k1"}})
				state.Get(vars.VariableDefinition{Ref: vars.VariableReference{Path: "k2"}})
				mapit := vars.TrackedVarsMap{}
				state.IterateInterpolatedCreds(mapit)
				Expect(mapit["k1"]).To(Equal("v1"))
				Expect(mapit["k2"]).To(Equal("v2"))
				// "k3" has not been Get, thus should not be tracked.
				Expect(mapit).ToNot(HaveKey("k3"))
			})
		})

		Context("when redaction is not enabled", func() {
			BeforeEach(func() {
				state = exec.NewRunState(credVars, false)
			})

			It("fetched variables are not tracked", func() {
				state.Get(vars.VariableDefinition{Ref: vars.VariableReference{Path: "k1"}})
				state.Get(vars.VariableDefinition{Ref: vars.VariableReference{Path: "k2"}})
				mapit := vars.TrackedVarsMap{}
				state.IterateInterpolatedCreds(mapit)
				Expect(mapit).ToNot(HaveKey("k1"))
				Expect(mapit).ToNot(HaveKey("k2"))
				Expect(mapit).ToNot(HaveKey("k3"))
			})
		})
	})

	Describe("List", func() {
		It("returns list of names from multiple vars with duplicates", func() {
			defs, err := state.List()
			Expect(defs).To(ConsistOf([]vars.VariableDefinition{
				{Ref: vars.VariableReference{Path: "k1"}},
				{Ref: vars.VariableReference{Path: "k2"}},
				{Ref: vars.VariableReference{Path: "k3"}},
			}))
			Expect(err).ToNot(HaveOccurred())
		})

		It("includes all local vars", func() {
			state.AddLocalVar("l1", 1, false)
			state.AddLocalVar("l2", 2, false)

			defs, err := state.List()
			Expect(defs).To(ConsistOf([]vars.VariableDefinition{
				{Ref: vars.VariableReference{Source: ".", Path: "l1"}},
				{Ref: vars.VariableReference{Source: ".", Path: "l2"}},

				{Ref: vars.VariableReference{Path: "k1"}},
				{Ref: vars.VariableReference{Path: "k2"}},
				{Ref: vars.VariableReference{Path: "k3"}},
			}))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("AddLocalVar", func() {
		Describe("redact", func() {
			BeforeEach(func() {
				state = exec.NewRunState(credVars, true)
				state.AddLocalVar("foo", "bar", true)
			})

			It("should get local value", func() {
				val, found, err := state.Get(vars.VariableDefinition{Ref: vars.VariableReference{Source: ".", Path: "foo"}})
				Expect(err).To(BeNil())
				Expect(found).To(BeTrue())
				Expect(val).To(Equal("bar"))
			})

			It("fetched variables are tracked when added", func() {
				mapit := vars.TrackedVarsMap{}
				state.IterateInterpolatedCreds(mapit)
				Expect(mapit["foo"]).To(Equal("bar"))
			})
		})

		Describe("not redact", func() {
			BeforeEach(func() {
				state.AddLocalVar("foo", "bar", false)
			})

			It("should get local value", func() {
				val, found, err := state.Get(vars.VariableDefinition{Ref: vars.VariableReference{Source: ".", Path: "foo"}})
				Expect(err).To(BeNil())
				Expect(found).To(BeTrue())
				Expect(val).To(Equal("bar"))
			})

			It("fetched variables are not tracked", func() {
				state.Get(vars.VariableDefinition{Ref: vars.VariableReference{Source: ".", Path: "foo"}})
				mapit := vars.TrackedVarsMap{}
				state.IterateInterpolatedCreds(mapit)
				Expect(mapit).ToNot(ContainElement("foo"))
			})
		})
	})

	Describe("NewLocalScope", func() {
		It("can access local vars from parent scope", func() {
			state.AddLocalVar("hello", "world", false)
			scope := state.NewLocalScope()
			val, _, _ := scope.Get(vars.VariableDefinition{Ref: vars.VariableReference{Source: ".", Path: "hello"}})
			Expect(val).To(Equal("world"))
		})

		It("adding local vars does not affect the original tracker", func() {
			scope := state.NewLocalScope()
			scope.AddLocalVar("hello", "world", false)
			_, found, _ := state.Get(vars.VariableDefinition{Ref: vars.VariableReference{Source: ".", Path: "hello"}})
			Expect(found).To(BeFalse())
		})

		It("shares the underlying non-local variables", func() {
			scope := state.NewLocalScope()
			val, _, _ := scope.Get(vars.VariableDefinition{Ref: vars.VariableReference{Path: "k1"}})
			Expect(val).To(Equal("v1"))
		})

		It("local vars added after creating the subscope are accessible", func() {
			scope := state.NewLocalScope()
			state.AddLocalVar("hello", "world", false)
			val, _, _ := scope.Get(vars.VariableDefinition{Ref: vars.VariableReference{Source: ".", Path: "hello"}})
			Expect(val).To(Equal("world"))
		})

		It("current scope is preferred over parent scope", func() {
			state.AddLocalVar("a", 1, false)
			scope := state.NewLocalScope()
			scope.AddLocalVar("a", 2, false)

			val, _, _ := scope.Get(vars.VariableDefinition{Ref: vars.VariableReference{Source: ".", Path: "a"}})
			Expect(val).To(Equal(2))
		})

		It("results set in parent scope are accessible in child", func() {
			parent := state
			child := parent.NewLocalScope()

			parent.StoreResult("id", "hello")

			var dst string
			child.Result("id", &dst)
			Expect(dst).To(Equal("hello"))
		})

		It("results set in child scope are accessible in parent", func() {
			parent := state
			child := parent.NewLocalScope()

			child.StoreResult("id", "hello")

			var dst string
			state.Result("id", &dst)
			Expect(dst).To(Equal("hello"))
		})

		It("shares the artifact repository between scopes", func() {
			parent := state
			child := parent.NewLocalScope()

			Expect(child.ArtifactRepository()).To(BeIdenticalTo(parent.ArtifactRepository()))
		})

		Describe("TrackedVarsMap", func() {
			BeforeEach(func() {
				state = exec.NewRunState(credVars, true)
			})

			It("prefers the value set in the current scope over the parent scope", func() {
				state.AddLocalVar("a", "from parent", true)
				scope := state.NewLocalScope()
				scope.AddLocalVar("a", "from child", true)

				mapit := vars.TrackedVarsMap{}
				scope.IterateInterpolatedCreds(mapit)

				Expect(mapit["a"]).To(Equal("from child"))
			})
		})
	})
})
