package atc_test

import (
	. "github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
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
			Ω(JobOutputConfig{}.PerformOn()).Should(Equal([]OutputCondition{"success"}))

			Ω(JobOutputConfig{
				RawPerformOn: []OutputCondition{},
			}.PerformOn()).Should(Equal([]OutputCondition{}))

			Ω(JobOutputConfig{
				RawPerformOn: []OutputCondition{"failure"},
			}.PerformOn()).Should(Equal([]OutputCondition{"failure"}))
		})
	})
})
