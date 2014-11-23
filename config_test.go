package atc_test

import (
	. "github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Describe("InputConfig", func() {
		It("defaults its name to the resource name", func() {
			Ω(InputConfig{
				Resource: "some-resource",
			}.Name()).Should(Equal("some-resource"))

			Ω(InputConfig{
				RawName:  "some-name",
				Resource: "some-resource",
			}.Name()).Should(Equal("some-name"))
		})

		It("defaults trigger to true", func() {
			Ω(InputConfig{}.Trigger()).Should(BeTrue())

			trigger := false
			Ω(InputConfig{RawTrigger: &trigger}.Trigger()).Should(BeFalse())

			trigger = true
			Ω(InputConfig{RawTrigger: &trigger}.Trigger()).Should(BeTrue())
		})
	})

	Describe("OutputConfig", func() {
		It("defaults PerformOn to [success]", func() {
			Ω(OutputConfig{}.PerformOn()).Should(Equal([]OutputCondition{"success"}))

			Ω(OutputConfig{
				RawPerformOn: []OutputCondition{},
			}.PerformOn()).Should(Equal([]OutputCondition{}))

			Ω(OutputConfig{
				RawPerformOn: []OutputCondition{"failure"},
			}.PerformOn()).Should(Equal([]OutputCondition{"failure"}))
		})
	})
})
