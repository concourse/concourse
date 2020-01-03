package gc_test

import (
	"github.com/concourse/concourse/atc"
	. "github.com/concourse/concourse/atc/gc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildLogRetentionCalculator", func() {
	It("nothing set gives all", func() {
		logRetention := NewBuildLogRetentionCalculator(0, 0, 0, 0).BuildLogsToRetain(makeJob(0, 0, 0))
		Expect(logRetention.Builds).To(Equal(0))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(0))
		Expect(logRetention.Days).To(Equal(0))
	})
	It("nothing set but job gives job", func() {
		logRetention := NewBuildLogRetentionCalculator(0, 0, 0, 0).BuildLogsToRetain(makeJob(3, 1, 0))
		Expect(logRetention.Builds).To(Equal(3))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(1))
		Expect(logRetention.Days).To(Equal(0))
	})
	It("default set gives default", func() {
		logRetention := NewBuildLogRetentionCalculator(5, 0, 0, 0).BuildLogsToRetain(makeJob(0, 0, 0))
		Expect(logRetention.Builds).To(Equal(5))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(0))
		Expect(logRetention.Days).To(Equal(0))
	})
	It("default and job set gives job", func() {
		logRetention := NewBuildLogRetentionCalculator(5, 0, 0, 0).BuildLogsToRetain(makeJob(6, 2, 0))
		Expect(logRetention.Builds).To(Equal(6))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(2))
		Expect(logRetention.Days).To(Equal(0))
	})
	It("default and job set and max set gives max if lower", func() {
		logRetention := NewBuildLogRetentionCalculator(5, 4, 0, 0).BuildLogsToRetain(makeJob(6, 7, 0))
		Expect(logRetention.Builds).To(Equal(4))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(0))
		Expect(logRetention.Days).To(Equal(0))
	})
	It("max only set gives max", func() {
		logRetention := NewBuildLogRetentionCalculator(0, 4, 0, 0).BuildLogsToRetain(makeJob(0, 6, 0))
		Expect(logRetention.Builds).To(Equal(4))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(0))
		Expect(logRetention.Days).To(Equal(0))
	})
	It("mix of count and days with max", func() {
		logRetention := NewBuildLogRetentionCalculator(2, 4, 3, 2).BuildLogsToRetain(makeJob(5, 8, 5))
		Expect(logRetention.Builds).To(Equal(4))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(0))
		Expect(logRetention.Days).To(Equal(2))
	})
	It("min success builds equals to builds", func() {
		logRetention := NewBuildLogRetentionCalculator(2, 10, 3, 0).BuildLogsToRetain(makeJob(5, 5, 0))
		Expect(logRetention.Builds).To(Equal(5))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(5))
		Expect(logRetention.Days).To(Equal(0))
	})
	It("min success builds greater than builds", func() {
		logRetention := NewBuildLogRetentionCalculator(2, 10, 3, 0).BuildLogsToRetain(makeJob(5, 8, 0))
		Expect(logRetention.Builds).To(Equal(5))
		Expect(logRetention.MinimumSucceededBuilds).To(Equal(0))
		Expect(logRetention.Days).To(Equal(0))
	})
})

func makeJob(retainAmount int, retainMinSuccessAmount, retainAmountDays int) atc.JobConfig {
	return atc.JobConfig{
		BuildLogRetention: &atc.BuildLogRetention{
			Builds:                 retainAmount,
			Days:                   retainAmountDays,
			MinimumSucceededBuilds: retainMinSuccessAmount,
		},
	}
}
