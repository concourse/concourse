package atc_test

import (
	"time"

	. "github.com/concourse/atc"
	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {

	Describe("JobConfig", func() {
		Describe("IsSerial", func() {
			It("returns true if Serial is true or SerialGroups has items in it", func() {
				jobConfig := JobConfig{
					Serial:       true,
					SerialGroups: []string{},
				}

				Ω(jobConfig.IsSerial()).Should(BeTrue())

				jobConfig.SerialGroups = []string{
					"one",
				}
				Ω(jobConfig.IsSerial()).Should(BeTrue())

				jobConfig.Serial = false
				Ω(jobConfig.IsSerial()).Should(BeTrue())
			})

			It("returns false if Serial is false and SerialGroups is empty", func() {
				jobConfig := JobConfig{
					Serial:       false,
					SerialGroups: []string{},
				}

				Ω(jobConfig.IsSerial()).Should(BeFalse())
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

		It("defaults trigger to true", func() {
			Ω(JobInputConfig{}.Trigger()).Should(BeTrue())

			trigger := false
			Ω(JobInputConfig{RawTrigger: &trigger}.Trigger()).Should(BeFalse())

			trigger = true
			Ω(JobInputConfig{RawTrigger: &trigger}.Trigger()).Should(BeTrue())
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

	Describe("Duration", func() {
		It("can be unmarshalled from YAML as a string", func() {
			var duration Duration
			err := yaml.Unmarshal([]byte("10s"), &duration)
			Expect(err).ToNot(HaveOccurred())

			Expect(time.Duration(duration)).To(Equal(10 * time.Second))
		})

		It("can be unmarshalled from YAML as an integer", func() {
			var duration Duration
			err := yaml.Unmarshal([]byte("10"), &duration)
			Expect(err).ToNot(HaveOccurred())

			Expect(time.Duration(duration)).To(Equal(10 * time.Nanosecond))
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
