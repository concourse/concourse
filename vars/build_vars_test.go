package vars_test

import (
	"github.com/concourse/concourse/vars"
	. "github.com/concourse/concourse/vars"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildVariables", func() {
	var buildVars *vars.BuildVariables

	Describe("turn on track", func() {
		BeforeEach(func() {
			buildVars = vars.NewBuildVariables(StaticVariables{"k1": "v1", "k2": "v2", "k3": "v3"}, true)
		})

		Describe("Get", func() {
			It("returns expected value", func() {
				val, found, err := buildVars.Get(VariableDefinition{Ref: VariableReference{Path: "k1"}})
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
				Expect(val).To(Equal("v1"))
			})

			It("fetched variables are tracked", func() {
				buildVars.Get(VariableDefinition{Ref: VariableReference{Path: "k1"}})
				buildVars.Get(VariableDefinition{Ref: VariableReference{Path: "k2"}})
				mapit := vars.TrackedVarsMap{}
				buildVars.IterateInterpolatedCreds(mapit)
				Expect(mapit["k1"]).To(Equal("v1"))
				Expect(mapit["k2"]).To(Equal("v2"))
				// "k3" has not been Get, thus should not be tracked.
				Expect(mapit).ToNot(HaveKey("k3"))
			})
		})

		Describe("List", func() {
			It("returns list of names from multiple vars with duplicates", func() {
				defs, err := buildVars.List()
				Expect(defs).To(ConsistOf([]VariableDefinition{
					{Ref: VariableReference{Path: "k1"}},
					{Ref: VariableReference{Path: "k2"}},
					{Ref: VariableReference{Path: "k3"}},
				}))
				Expect(err).ToNot(HaveOccurred())
			})

			It("includes all local vars", func() {
				buildVars.AddLocalVar("l1", 1, false)
				buildVars.AddLocalVar("l2", 2, false)

				defs, err := buildVars.List()
				Expect(defs).To(ConsistOf([]VariableDefinition{
					{Ref: VariableReference{Source: ".", Path: "l1"}},
					{Ref: VariableReference{Source: ".", Path: "l2"}},

					{Ref: VariableReference{Path: "k1"}},
					{Ref: VariableReference{Path: "k2"}},
					{Ref: VariableReference{Path: "k3"}},
				}))
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Describe("AddLocalVar", func() {
			Describe("redact", func() {
				BeforeEach(func() {
					buildVars.AddLocalVar("foo", "bar", true)
				})

				It("should get local value", func() {
					val, found, err := buildVars.Get(VariableDefinition{Ref: VariableReference{Source: ".", Path: "foo"}})
					Expect(err).To(BeNil())
					Expect(found).To(BeTrue())
					Expect(val).To(Equal("bar"))
				})

				It("fetched variables are tracked when added", func() {
					mapit := vars.TrackedVarsMap{}
					buildVars.IterateInterpolatedCreds(mapit)
					Expect(mapit["foo"]).To(Equal("bar"))
				})
			})
		})

		Describe("NewLocalScope", func() {
			It("can access local vars from parent scope", func() {
				buildVars.AddLocalVar("hello", "world", false)
				scope := buildVars.NewLocalScope()
				val, _, _ := scope.Get(VariableDefinition{Ref: VariableReference{Source: ".", Path: "hello"}})
				Expect(val).To(Equal("world"))
			})

			It("adding local vars does not affect the original tracker", func() {
				scope := buildVars.NewLocalScope()
				scope.AddLocalVar("hello", "world", false)
				_, found, _ := buildVars.Get(VariableDefinition{Ref: VariableReference{Source: ".", Path: "hello"}})
				Expect(found).To(BeFalse())
			})

			It("shares the underlying non-local variables", func() {
				scope := buildVars.NewLocalScope()
				val, _, _ := scope.Get(VariableDefinition{Ref: VariableReference{Path: "k1"}})
				Expect(val).To(Equal("v1"))
			})

			It("local vars added after creating the subscope are accessible", func() {
				scope := buildVars.NewLocalScope()
				buildVars.AddLocalVar("hello", "world", false)
				val, _, _ := scope.Get(VariableDefinition{Ref: VariableReference{Source: ".", Path: "hello"}})
				Expect(val).To(Equal("world"))
			})

			It("current scope is preferred over parent scope", func() {
				buildVars.AddLocalVar("a", 1, false)
				scope := buildVars.NewLocalScope()
				scope.AddLocalVar("a", 2, false)

				val, _, _ := scope.Get(VariableDefinition{Ref: VariableReference{Source: ".", Path: "a"}})
				Expect(val).To(Equal(2))
			})

			Describe("TrackedVarsMap", func() {
				It("prefers the value set in the current scope over the parent scope", func() {
					buildVars.AddLocalVar("a", "from parent", true)
					scope := buildVars.NewLocalScope()
					scope.AddLocalVar("a", "from child", true)

					mapit := vars.TrackedVarsMap{}
					scope.IterateInterpolatedCreds(mapit)

					Expect(mapit["a"]).To(Equal("from child"))
				})
			})
		})

		Describe("not redact", func() {
			BeforeEach(func() {
				buildVars.AddLocalVar("foo", "bar", false)
			})

			It("should get local value", func() {
				val, found, err := buildVars.Get(VariableDefinition{Ref: VariableReference{Source: ".", Path: "foo"}})
				Expect(err).To(BeNil())
				Expect(found).To(BeTrue())
				Expect(val).To(Equal("bar"))
			})

			It("fetched variables are not tracked", func() {
				buildVars.Get(VariableDefinition{Ref: VariableReference{Source: ".", Path: "foo"}})
				mapit := vars.TrackedVarsMap{}
				buildVars.IterateInterpolatedCreds(mapit)
				Expect(mapit).ToNot(ContainElement("foo"))
			})
		})
	})

	Describe("turn off track", func() {
		BeforeEach(func() {
			buildVars = vars.NewBuildVariables(StaticVariables{"k1": "v1", "k2": "v2", "k3": "v3"}, false)
		})

		Describe("Get", func() {
			It("returns expected value", func() {
				val, found, err := buildVars.Get(VariableDefinition{Ref: VariableReference{Path: "k1"}})
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
				Expect(val).To(Equal("v1"))
			})

			It("fetched variables should not be tracked", func() {
				buildVars.Get(VariableDefinition{Ref: VariableReference{Path: "k1"}})
				buildVars.Get(VariableDefinition{Ref: VariableReference{Path: "k2"}})
				mapit := vars.TrackedVarsMap{}
				buildVars.IterateInterpolatedCreds(mapit)
				Expect(mapit).ToNot(HaveKey("k1"))
				Expect(mapit).ToNot(HaveKey("k2"))
				Expect(mapit).ToNot(HaveKey("k3"))
			})
		})

		Describe("List", func() {
			It("returns list of names from multiple vars with duplicates", func() {
				defs, err := buildVars.List()
				Expect(defs).To(ConsistOf([]VariableDefinition{
					{Ref: VariableReference{Path: "k1"}},
					{Ref: VariableReference{Path: "k2"}},
					{Ref: VariableReference{Path: "k3"}},
				}))
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
