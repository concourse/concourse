package gc_test

import (
	"github.com/concourse/concourse/atc"
	. "github.com/concourse/concourse/atc/gc"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildLogRetentionCalculator", func() {
	It("nothing set returns zeros", func() {
		logRetention := NewBuildLogRetentionCalculator(
			0, // default builds to retain
			0, // max builds to retain
			0, // default days to retain
			0, // max days to retain
		).BuildLogsToRetain(makeJob(
			0, // builds to retain
			0, // days to retain
			0, // min success to retain
		))
		Expect(logRetention.Builds).To(Equal(0))
		Expect(logRetention.Days).To(Equal(0))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(0))
	})
	It("no default or max set, returns job values", func() {
		logRetention := NewBuildLogRetentionCalculator(
			0, // default builds to retain
			0, // max builds to retain
			0, // default days to retain
			0, // max days to retain
		).BuildLogsToRetain(makeJob(
			3, // builds to retain
			2, // days to retain
			1, // min success to retain
		))
		Expect(logRetention.Builds).To(Equal(3))
		Expect(logRetention.Days).To(Equal(2))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(1))
	})
	It("default set gives default", func() {
		logRetention := NewBuildLogRetentionCalculator(
			5, // default builds to retain
			0, // max builds to retain
			4, // default days to retain
			0, // max days to retain
		).BuildLogsToRetain(makeJob(
			0, // builds to retain
			0, // days to retain
			0, // min success to retain
		))
		Expect(logRetention.Builds).To(Equal(5))
		Expect(logRetention.Days).To(Equal(4))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(0))
	})
	It("default and job set gives job", func() {
		logRetention := NewBuildLogRetentionCalculator(
			5, // default builds to retain
			0, // max builds to retain
			4, // default days to retain
			0, // max days to retain
		).BuildLogsToRetain(makeJob(
			6, // builds to retain
			3, // days to retain
			0, // min success to retain
		))
		Expect(logRetention.Builds).To(Equal(6))
		Expect(logRetention.Days).To(Equal(3))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(0))
	})
	It("default, max, and job set, gives max if lower", func() {
		logRetention := NewBuildLogRetentionCalculator(
			5, // default builds to retain
			6, // max builds to retain
			5, // default days to retain
			6, // max days to retain
		).BuildLogsToRetain(makeJob(
			10, // builds to retain
			9,  // days to retain
			0,  // min success to retain
		))
		Expect(logRetention.Builds).To(Equal(6))
		Expect(logRetention.Days).To(Equal(6))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(0))
	})
	It("max only set gives max", func() {
		logRetention := NewBuildLogRetentionCalculator(
			0, // default builds to retain
			4, // max builds to retain
			0, // default days to retain
			3, // max days to retain
		).BuildLogsToRetain(makeJob(
			0, // builds to retain
			0, // days to retain
			0, // min success to retain
		))
		Expect(logRetention.Builds).To(Equal(4))
		Expect(logRetention.Days).To(Equal(3))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(0))
	})
	It("mix of count and days with max", func() {
		logRetention := NewBuildLogRetentionCalculator(
			2, // default builds to retain
			4, // max builds to retain
			3, // default days to retain
			2, // max days to retain
		).BuildLogsToRetain(makeJob(
			5, // builds to retain
			5, // days to retain
			8, // min success to retain
		))
		Expect(logRetention.Builds).To(Equal(4))
		Expect(logRetention.Days).To(Equal(2))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(0))
	})
	It("min success builds equals to builds", func() {
		logRetention := NewBuildLogRetentionCalculator(
			2,  // default builds to retain
			10, // max builds to retain
			3,  // default days to retain
			0,  // max days to retain
		).BuildLogsToRetain(makeJob(
			5, // builds to retain
			0, // days to retain
			5, // min success to retain
		))
		Expect(logRetention.Builds).To(Equal(5))
		Expect(logRetention.Days).To(Equal(3))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(5))
	})
	It("min success builds greater than builds", func() {
		logRetention := NewBuildLogRetentionCalculator(
			2,  // default builds to retain
			10, // max builds to retain
			3,  // default days to retain
			0,  // max days to retain
		).BuildLogsToRetain(makeJob(
			5, // builds to retain
			0, // days to retain
			8, // min success to retain
		))
		Expect(logRetention.Builds).To(Equal(5))
		Expect(logRetention.Days).To(Equal(3))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(0))
	})
	It("when only max builds is set and job build and days are set", func() {
		logRetention := NewBuildLogRetentionCalculator(
			0, // default builds to retain
			7, // max builds to retain
			0, // default days to retain
			0, // max days to retain
		).BuildLogsToRetain(makeJob(
			5, // builds to retain
			7, // days to retain
			0, // min success to retain
		))
		Expect(logRetention.Builds).To(Equal(5))
		Expect(logRetention.Days).To(Equal(7))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(0))
	})
	It("when only max days is set and job build and days are set", func() {
		logRetention := NewBuildLogRetentionCalculator(
			0, // default builds to retain
			0, // max builds to retain
			0, // default days to retain
			7, // max days to retain
		).BuildLogsToRetain(makeJob(
			7, // builds to retain
			5, // days to retain
			0, // min success to retain
		))
		Expect(logRetention.Builds).To(Equal(7))
		Expect(logRetention.Days).To(Equal(5))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(0))
	})
})

func makeJob(retainAmount, retainAmountDays, retainMinSuccessAmount int) atc.JobConfig {
	return atc.JobConfig{
		BuildLogRetention: &atc.BuildLogRetention{
			Builds:                 retainAmount,
			Days:                   retainAmountDays,
			MinimumSucceededBuilds: retainMinSuccessAmount,
		},
	}
}
