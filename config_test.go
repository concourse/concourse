package atc_test

import (
	. "github.com/concourse/atc"

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

})
