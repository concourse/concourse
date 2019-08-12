package atc_test

import (
	"encoding/json"

	. "github.com/concourse/concourse/atc"
	"github.com/ghodss/yaml"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Describe("VersionConfig", func() {
		Context("when unmarshaling a pinned version from YAML", func() {
			It("produces the correct version config without error", func() {
				var versionConfig VersionConfig
				bs := []byte(`some: version`)
				err := yaml.Unmarshal(bs, &versionConfig)
				Expect(err).NotTo(HaveOccurred())

				expected := VersionConfig{
					Pinned: Version{
						"some": "version",
					},
				}

				Expect(versionConfig).To(Equal(expected))
			})
		})

		Context("when unmarshaling a pinned version from JSON", func() {
			It("produces the correct version config without error", func() {
				var versionConfig VersionConfig
				bs := []byte(`{ "some": "version" }`)
				err := json.Unmarshal(bs, &versionConfig)
				Expect(err).NotTo(HaveOccurred())

				expected := VersionConfig{
					Pinned: Version{
						"some": "version",
					},
				}

				Expect(versionConfig).To(Equal(expected))
			})
		})
	})
})
