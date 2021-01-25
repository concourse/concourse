package build_test

import (
	"github.com/concourse/concourse/atc"
	. "github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/vars"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Variables", func() {
	var (
		buildVars  *Variables
		varSources atc.VarSourceConfigs
	)

	BeforeEach(func() {
		varSources = atc.VarSourceConfigs{
			{
				Name:   "some-var-source",
				Type:   "registry",
				Config: map[string]string{"some": "config"},
			},
		}
	})

	Describe("Get", func() {
		BeforeEach(func() {
			buildVars = NewVariables(varSources, false)
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

		Context("when redaction is enabled", func() {
			BeforeEach(func() {
				buildVars = NewVariables(varSources, true)
				buildVars.SetVar(".", "k1", "v1", true)
				buildVars.SetVar(".", "k2", "v2", true)
			})

			It("fetched variables are tracked", func() {
				mapit := vars.TrackedVarsMap{}
				buildVars.IterateInterpolatedCreds(mapit)
				Expect(mapit["k1"]).To(Equal("v1"))
				Expect(mapit["k2"]).To(Equal("v2"))
				// "k3" has not been Get, thus should not be tracked.
				Expect(mapit).ToNot(HaveKey("k3"))
			})
		})

		Context("when redaction is not enabled", func() {
			BeforeEach(func() {
				buildVars = NewVariables(varSources, false)
				buildVars.SetVar(".", "k1", "v1", false)
				buildVars.SetVar(".", "k2", "v2", false)
			})

			It("fetched variables are not tracked", func() {
				mapit := vars.TrackedVarsMap{}
				buildVars.IterateInterpolatedCreds(mapit)
				Expect(mapit).ToNot(HaveKey("k1"))
				Expect(mapit).ToNot(HaveKey("k2"))
				Expect(mapit).ToNot(HaveKey("k3"))
			})
		})
	})

	Describe("SetVar", func() {
		Describe("redact", func() {
			BeforeEach(func() {
				buildVars = NewVariables(varSources, true)
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
				buildVars.IterateInterpolatedCreds(mapit)
				Expect(mapit["foo"]).To(Equal("bar"))
			})
		})

		Describe("not redact", func() {
			BeforeEach(func() {
				buildVars = NewVariables(varSources, true)
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
				buildVars.IterateInterpolatedCreds(mapit)
				Expect(mapit).ToNot(ContainElement("foo"))
			})
		})
	})
})
