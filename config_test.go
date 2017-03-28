package atc_test

import (
	"encoding/json"

	. "github.com/concourse/atc"
	yaml "gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Describe("JobConfig", func() {
		Describe("MaxInFlight", func() {
			It("returns the raw MaxInFlight if set", func() {
				jobConfig := JobConfig{
					RawMaxInFlight: 42,
				}

				Expect(jobConfig.MaxInFlight()).To(Equal(42))
			})

			It("returns 1 if Serial is true or SerialGroups has items in it", func() {
				jobConfig := JobConfig{
					Serial:       true,
					SerialGroups: []string{},
				}

				Expect(jobConfig.MaxInFlight()).To(Equal(1))

				jobConfig.SerialGroups = []string{
					"one",
				}
				Expect(jobConfig.MaxInFlight()).To(Equal(1))

				jobConfig.Serial = false
				Expect(jobConfig.MaxInFlight()).To(Equal(1))
			})

			It("returns 1 if Serial is true or SerialGroups has items in it, even if raw MaxInFlight is set", func() {
				jobConfig := JobConfig{
					Serial:         true,
					SerialGroups:   []string{},
					RawMaxInFlight: 3,
				}

				Expect(jobConfig.MaxInFlight()).To(Equal(1))

				jobConfig.SerialGroups = []string{
					"one",
				}
				Expect(jobConfig.MaxInFlight()).To(Equal(1))

				jobConfig.Serial = false
				Expect(jobConfig.MaxInFlight()).To(Equal(1))
			})

			It("returns 0 if MaxInFlight is not set, Serial is false, and SerialGroups is empty", func() {
				jobConfig := JobConfig{
					Serial:       false,
					SerialGroups: []string{},
				}

				Expect(jobConfig.MaxInFlight()).To(Equal(0))
			})
		})

		Describe("GetSerialGroups", func() {
			It("Returns the values if SerialGroups is specified", func() {
				jobConfig := JobConfig{
					SerialGroups: []string{"one", "two"},
				}

				Expect(jobConfig.GetSerialGroups()).To(Equal([]string{"one", "two"}))
			})

			It("Returns the job name if Serial but SerialGroups are not specified", func() {
				jobConfig := JobConfig{
					Name:   "some-job",
					Serial: true,
				}

				Expect(jobConfig.GetSerialGroups()).To(Equal([]string{"some-job"}))
			})

			It("Returns the job name if MaxInFlight but SerialGroups are not specified", func() {
				jobConfig := JobConfig{
					Name:           "some-job",
					RawMaxInFlight: 1,
				}

				Expect(jobConfig.GetSerialGroups()).To(Equal([]string{"some-job"}))
			})

			It("returns an empty slice of strings if there are no groups and it is not serial and has no max-in-flight", func() {
				jobConfig := JobConfig{
					Name:   "some-job",
					Serial: false,
				}

				Expect(jobConfig.GetSerialGroups()).To(Equal([]string{}))
			})
		})
	})

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
