package atc_test

import (
	"encoding/json"
	"time"

	. "github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo/v2"
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

	Describe("CheckEvery", func() {
		Context("when unmarshaling", func() {
			Context("check_every is never", func() {
				It("parses as a bool without error", func() {
					var resourceConfig ResourceConfig
					bs := []byte(`{ "check_every": "never" }`)
					err := json.Unmarshal(bs, &resourceConfig)
					Expect(err).NotTo(HaveOccurred())

					expected := ResourceConfig{
						CheckEvery: &CheckEvery{Never: true},
					}

					Expect(resourceConfig).To(Equal(expected))
				})
			})

			Context("check_every is a duration", func() {
				It("parses as a duration without error", func() {
					var resourceConfig ResourceConfig
					bs := []byte(`{ "check_every": "10s" }`)
					err := json.Unmarshal(bs, &resourceConfig)
					Expect(err).NotTo(HaveOccurred())

					expected := ResourceConfig{
						CheckEvery: &CheckEvery{Interval: 10 * time.Second},
					}

					Expect(resourceConfig).To(Equal(expected))
				})
			})

			Context("check_every is a non-duration string", func() {
				It("errors", func() {
					var resourceConfig ResourceConfig
					bs := []byte(`{ "check_every": "some-string" }`)
					err := json.Unmarshal(bs, &resourceConfig)
					Expect(err).To(MatchError(`time: invalid duration "some-string"`))
				})
			})

			Context("check_every is not a string", func() {
				It("errors", func() {
					var resourceConfig ResourceConfig
					bs := []byte(`{ "check_every": [1,2,3] }`)
					err := json.Unmarshal(bs, &resourceConfig)
					Expect(err).To(MatchError("non-string value provided"))
				})
			})
		})

		Context("marshaling", func() {
			Context("never is true", func() {
				It("returns never", func() {
					checkEvery := CheckEvery{Never: true}
					result, err := checkEvery.MarshalJSON()
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal([]byte(`"never"`)))
				})
			})
			Context("interval is set", func() {
				It("returns the duration as a string", func() {
					checkEvery := CheckEvery{Interval: 10 * time.Second}
					result, err := checkEvery.MarshalJSON()
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal([]byte(`"10s"`)))
				})
			})
			Context("both never and interval are not set", func() {
				It("returns an empty byte", func() {
					checkEvery := CheckEvery{}
					result, err := checkEvery.MarshalJSON()
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal([]byte(`""`)))
				})
			})
		})
	})
})
