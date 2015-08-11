package atc_test

import (
	. "github.com/concourse/atc"
	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Describe("JobConfig", func() {
		Describe("MaxInFlight", func() {
			It("returns the raw MaxInFlight if set", func() {
				jobConfig := JobConfig{
					Serial:         true,
					RawMaxInFlight: 42,
				}

				Ω(jobConfig.MaxInFlight()).Should(Equal(42))
			})

			It("returns 1 if Serial is true or SerialGroups has items in it", func() {
				jobConfig := JobConfig{
					Serial:       true,
					SerialGroups: []string{},
				}

				Ω(jobConfig.MaxInFlight()).Should(Equal(1))

				jobConfig.SerialGroups = []string{
					"one",
				}
				Ω(jobConfig.MaxInFlight()).Should(Equal(1))

				jobConfig.Serial = false
				Ω(jobConfig.MaxInFlight()).Should(Equal(1))
			})

			It("returns 0 if MaxInFlight is not set, Serial is false, and SerialGroups is empty", func() {
				jobConfig := JobConfig{
					Serial:       false,
					SerialGroups: []string{},
				}

				Ω(jobConfig.MaxInFlight()).Should(Equal(0))
			})
		})

		Describe("GetSerialGroups", func() {
			It("Returns the values if SerialGroups is specified", func() {
				jobConfig := JobConfig{
					SerialGroups: []string{"one", "two"},
				}

				Ω(jobConfig.GetSerialGroups()).Should(Equal([]string{"one", "two"}))
			})

			It("Returns the job name if Serial but SerialGroups are not specified", func() {
				jobConfig := JobConfig{
					Name:   "some-job",
					Serial: true,
				}

				Ω(jobConfig.GetSerialGroups()).Should(Equal([]string{"some-job"}))
			})

			It("Returns the job name if MaxInFlight but SerialGroups are not specified", func() {
				jobConfig := JobConfig{
					Name:           "some-job",
					RawMaxInFlight: 1,
				}

				Ω(jobConfig.GetSerialGroups()).Should(Equal([]string{"some-job"}))
			})

			It("returns an empty slice of strings if there are no groups and it is not serial and has no max-in-flight", func() {
				jobConfig := JobConfig{
					Name:   "some-job",
					Serial: false,
				}

				Ω(jobConfig.GetSerialGroups()).Should(Equal([]string{}))
			})
		})
	})

	Describe("JobInputConfig", func() {
		It("defaults its name to the resource name", func() {
			Ω(JobInputConfig{
				Resource: "some-resource",
			}.Name()).Should(Equal("some-resource"))

			Ω(JobInputConfig{
				RawName:  "some-name",
				Resource: "some-resource",
			}.Name()).Should(Equal("some-name"))
		})
	})

	Describe("JobOutputConfig", func() {
		It("defaults PerformOn to [success]", func() {
			Ω(JobOutputConfig{}.PerformOn()).Should(Equal([]Condition{"success"}))

			Ω(JobOutputConfig{
				RawPerformOn: []Condition{},
			}.PerformOn()).Should(Equal([]Condition{}))

			Ω(JobOutputConfig{
				RawPerformOn: []Condition{"failure"},
			}.PerformOn()).Should(Equal([]Condition{"failure"}))
		})
	})

	Describe("Condition", func() {
		It("can be unmarshalled from YAML as the string 'success'", func() {
			var condition Condition
			err := yaml.Unmarshal([]byte("success"), &condition)
			Expect(err).ToNot(HaveOccurred())

			Expect(condition).To(Equal(ConditionSuccess))
		})

		It("can be unmarshalled from YAML as the string 'failure'", func() {
			var condition Condition
			err := yaml.Unmarshal([]byte("failure"), &condition)
			Expect(err).ToNot(HaveOccurred())

			Expect(condition).To(Equal(ConditionFailure))
		})

		It("fails to unmarshal other strings", func() {
			var condition Condition
			err := yaml.Unmarshal([]byte("bogus"), &condition)
			Expect(err).To(HaveOccurred())
		})
	})
})
