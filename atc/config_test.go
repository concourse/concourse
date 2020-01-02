package atc_test

import (
	"encoding/json"

	. "github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Describe("VersionConfig", func() {
		Context("when unmarshaling a pinned version from JSON", func() {
			Context("when the version is all string", func() {
				It("produces the correct version config without error", func() {
					var versionConfig VersionConfig
					bs := []byte(`{ "some": "version", "other": "8" }`)
					err := json.Unmarshal(bs, &versionConfig)
					Expect(err).NotTo(HaveOccurred())

					expected := VersionConfig{
						Pinned: Version{
							"some":  "version",
							"other": "8",
						},
					}

					Expect(versionConfig).To(Equal(expected))
				})
			})

			Context("when the version contains not all string", func() {
				It("produces an error", func() {
					var versionConfig VersionConfig
					bs := []byte(`{ "some": 8 }`)
					err := json.Unmarshal(bs, &versionConfig)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("the value 8 of some is not a string"))
				})
			})
		})
	})

	Describe("VarSourceConfigs.OrderByDependency", func() {
		var (
			varSources VarSourceConfigs
			ordered    VarSourceConfigs
			orderError error
		)

		vs1 := VarSourceConfig{
			Name: "vs1",
			Type: "dummy",
			Config: map[string]interface{}{
				"vars": map[string]interface{}{"pk": "pv"},
			},
		}
		vs1_5 := VarSourceConfig{
			Name: "vs1",
			Type: "dummy",
			Config: map[string]interface{}{
				"vars": map[string]interface{}{"pk": "((vs5:pk))"},
			},
		}
		vs2 := VarSourceConfig{
			Name: "vs2",
			Type: "dummy",
			Config: map[string]interface{}{
				"vars": map[string]interface{}{"pk": "pv"},
			},
		}
		vs3 := VarSourceConfig{
			Name: "vs3",
			Type: "dummy",
			Config: map[string]interface{}{
				"vars": map[string]interface{}{"pk": "((vs1:pk))"},
			},
		}
		vs4 := VarSourceConfig{
			Name: "vs4",
			Type: "dummy",
			Config: map[string]interface{}{
				"vars": map[string]interface{}{"pk": "((vs2:pk))"},
			},
		}
		vs5 := VarSourceConfig{
			Name: "vs5",
			Type: "dummy",
			Config: map[string]interface{}{
				"vars": map[string]interface{}{"pk": "((vs3:pk))", "pk2": "((vs4:pk))"},
			},
		}

		JustBeforeEach(func() {
			ordered, orderError = varSources.OrderByDependency()
		})

		Context("var_sources with ideal order", func() {
			BeforeEach(func() {
				varSources = VarSourceConfigs{vs1, vs2, vs3, vs4, vs5}
			})
			It("should keep the original order", func() {
				Expect(orderError).ToNot(HaveOccurred())
				Expect(ordered[0].Name).To(Equal("vs1"))
				Expect(ordered[1].Name).To(Equal("vs2"))
				Expect(ordered[2].Name).To(Equal("vs3"))
				Expect(ordered[3].Name).To(Equal("vs4"))
				Expect(ordered[4].Name).To(Equal("vs5"))
			})
		})

		Context("var_sources with random order", func() {
			BeforeEach(func() {
				varSources = VarSourceConfigs{vs4, vs2, vs5, vs1, vs3}
			})
			It("should order properly", func() {
				Expect(orderError).ToNot(HaveOccurred())
				Expect(ordered[0].Name).To(Equal("vs2"))
				Expect(ordered[1].Name).To(Equal("vs4"))
				Expect(ordered[2].Name).To(Equal("vs1"))
				Expect(ordered[3].Name).To(Equal("vs3"))
				Expect(ordered[4].Name).To(Equal("vs5"))
			})
		})

		Context("var_sources with unresolved dependency", func() {
			BeforeEach(func() {
				varSources = VarSourceConfigs{vs4, vs2, vs5, vs3}
			})
			It("should raise error", func() {
				Expect(orderError).To(HaveOccurred())
				Expect(orderError.Error()).To(Equal("could not resolve inter-dependent var sources: vs5, vs3"))
			})
		})

		Context("var_sources with cyclic dependencies", func() {
			BeforeEach(func() {
				varSources = VarSourceConfigs{vs1_5, vs4, vs2, vs5, vs3}
			})
			It("should raise error", func() {
				Expect(orderError).To(HaveOccurred())
				Expect(orderError.Error()).To(Equal("could not resolve inter-dependent var sources: vs1, vs5, vs3"))
			})
		})
	})
})
