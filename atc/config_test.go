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
					bs := []byte(`{ "some": "  version  ", "other": "8" }`)
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
})
