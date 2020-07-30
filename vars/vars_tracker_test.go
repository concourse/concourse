package vars_test

import (
	. "github.com/concourse/concourse/vars"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("vars_tracker", func() {
	var tracker CredVarsTracker

	Describe("turn on track", func() {
		BeforeEach(func() {
			v := StaticVariables{"k1": "v1", "k2": "v2", "k3": "v3"}
			tracker = NewCredVarsTracker(v, true)
		})

		Describe("Get", func() {
			It("returns expected value", func() {
				var (
					val   interface{}
					found bool
					err   error
				)
				val, found, err = tracker.Get(VariableDefinition{Ref: VariableReference{Path: "k1"}})
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
				Expect(val).To(Equal("v1"))
			})

			It("fetched variables are tracked", func() {
				tracker.Get(VariableDefinition{Ref: VariableReference{Path: "k1"}})
				tracker.Get(VariableDefinition{Ref: VariableReference{Path: "k2"}})
				mapit := NewMapCredVarsTrackerIterator()
				tracker.IterateInterpolatedCreds(mapit)
				Expect(mapit.Data["k1"]).To(Equal("v1"))
				Expect(mapit.Data["k2"]).To(Equal("v2"))
				// "k3" has not been Get, thus should not be tracked.
				Expect(mapit.Data["k3"]).To(BeNil())
			})
		})

		Describe("List", func() {
			It("returns list of names from multiple vars with duplicates", func() {
				defs, err := tracker.List()
				Expect(defs).To(ConsistOf([]VariableDefinition{
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
					tracker.AddLocalVar("foo", "bar", true)
				})

				It("should get local value", func() {
					var (
						val   interface{}
						found bool
						err   error
					)
					val, found, err = tracker.Get(VariableDefinition{Ref: VariableReference{Source: ".", Path: "foo"}})
					Expect(err).To(BeNil())
					Expect(found).To(BeTrue())
					Expect(val).To(Equal("bar"))
				})

				It("fetched variables are tracked", func() {
					tracker.Get(VariableDefinition{Ref: VariableReference{Source: ".", Path: "foo"}})
					mapit := NewMapCredVarsTrackerIterator()
					tracker.IterateInterpolatedCreds(mapit)
					Expect(mapit.Data["foo"]).To(Equal("bar"))
				})
			})
		})

		Describe("not redact", func() {
			BeforeEach(func() {
				tracker.AddLocalVar("foo", "bar", false)
			})

			It("should get local value", func() {
				var (
					val   interface{}
					found bool
					err   error
				)
				val, found, err = tracker.Get(VariableDefinition{Ref: VariableReference{Source: ".", Path: "foo"}})
				Expect(err).To(BeNil())
				Expect(found).To(BeTrue())
				Expect(val).To(Equal("bar"))
			})

			It("fetched variables are not tracked", func() {
				tracker.Get(VariableDefinition{Ref: VariableReference{Source: ".", Path: "foo"}})
				mapit := NewMapCredVarsTrackerIterator()
				tracker.IterateInterpolatedCreds(mapit)
				Expect(mapit.Data["foo"]).To(BeNil())
			})
		})
	})

	Describe("turn off track", func() {
		BeforeEach(func() {
			v := StaticVariables{"k1": "v1", "k2": "v2", "k3": "v3"}
			tracker = NewCredVarsTracker(v, false)
		})

		Describe("Get", func() {
			It("returns expected value", func() {
				var (
					val   interface{}
					found bool
					err   error
				)
				val, found, err = tracker.Get(VariableDefinition{Ref: VariableReference{Path: "k1"}})
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
				Expect(val).To(Equal("v1"))
			})

			It("fetched variables should not be tracked", func() {
				tracker.Get(VariableDefinition{Ref: VariableReference{Path: "k1"}})
				tracker.Get(VariableDefinition{Ref: VariableReference{Path: "k2"}})
				mapit := NewMapCredVarsTrackerIterator()
				tracker.IterateInterpolatedCreds(mapit)
				Expect(mapit.Data["k1"]).To(BeNil())
				Expect(mapit.Data["k2"]).To(BeNil())
				Expect(mapit.Data["k3"]).To(BeNil())
			})
		})

		Describe("List", func() {
			It("returns list of names from multiple vars with duplicates", func() {
				defs, err := tracker.List()
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
