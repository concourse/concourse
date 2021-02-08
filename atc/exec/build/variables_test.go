package build_test

import (
	. "github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/vars"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Variables", func() {
	var (
		buildVars *Variables
		tracker   *vars.Tracker
	)

	Describe("Get", func() {
		BeforeEach(func() {
			tracker = vars.NewTracker(false)
			buildVars = NewVariables(tracker)
			buildVars.SetVar(".", "k1", "v1", false)
		})

		It("fetches from cred vars", func() {
			val, found, err := buildVars.Get(vars.Reference{Source: ".", Path: "k1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(val).To(Equal("v1"))
		})

		Context("when local var field does not exist", func() {
			It("errors", func() {
				By("set variable ((.:foo.bar=baz))")
				buildVars.SetVar(".", "foo", map[string]interface{}{"bar": "baz"}, false)
				By("get variable ((.:foo.missing))")
				_, _, err := buildVars.Get(vars.Reference{Source: ".", Path: "foo", Fields: []string{"missing"}})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("SetVar", func() {
		Describe("redact", func() {
			BeforeEach(func() {
				tracker = vars.NewTracker(true)
				buildVars = NewVariables(tracker)
				buildVars.SetVar(".", "foo", "bar", true)
			})

			It("should store the value in the var", func() {
				val, found, err := buildVars.Get(vars.Reference{Source: ".", Path: "foo"})
				Expect(err).To(BeNil())
				Expect(found).To(BeTrue())
				Expect(val).To(Equal("bar"))
			})

			It("fetched variables are tracked when added", func() {
				mapit := vars.TrackedVarsMap{}
				tracker.IterateInterpolatedCreds(mapit)
				Expect(mapit[".:foo"]).To(Equal("bar"))
			})
		})

		Describe("not redact", func() {
			BeforeEach(func() {
				tracker = vars.NewTracker(true)
				buildVars = NewVariables(tracker)
				buildVars.SetVar(".", "foo", "bar", false)
			})

			It("should store the value in the var", func() {
				val, found, err := buildVars.Get(vars.Reference{Source: ".", Path: "foo"})
				Expect(err).To(BeNil())
				Expect(found).To(BeTrue())
				Expect(val).To(Equal("bar"))
			})

			It("fetched variables are not tracked", func() {
				buildVars.Get(vars.Reference{Source: ".", Path: "foo"})
				mapit := vars.TrackedVarsMap{}
				tracker.IterateInterpolatedCreds(mapit)
				Expect(mapit).ToNot(ContainElement(".:foo"))
			})
		})
	})
})
