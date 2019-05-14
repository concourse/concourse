package gc_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/gc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildLogRetentionCalculator", func() {
	It("nothing set gives all", func() {
		logRetention := NewBuildLogRetentionCalculator(0, 0, 0, 0).BuildLogsToRetain(makeJob(0, 0))
		Expect(logRetention.Builds).To(Equal(0))
		Expect(logRetention.Days).To(Equal(0))
	})
	It("nothing set but job gives job", func() {
		logRetention := NewBuildLogRetentionCalculator(0, 0, 0, 0).BuildLogsToRetain(makeJob(3, 0))
		Expect(logRetention.Builds).To(Equal(3))
		Expect(logRetention.Days).To(Equal(0))
	})
	It("default set gives default", func() {
		logRetention := NewBuildLogRetentionCalculator(5, 0, 0, 0).BuildLogsToRetain(makeJob(0, 0))
		Expect(logRetention.Builds).To(Equal(5))
		Expect(logRetention.Days).To(Equal(0))
	})
	It("default and job set gives job", func() {
		logRetention := NewBuildLogRetentionCalculator(5, 0, 0, 0).BuildLogsToRetain(makeJob(6, 0))
		Expect(logRetention.Builds).To(Equal(6))
		Expect(logRetention.Days).To(Equal(0))
	})
	It("default and job set and max set gives max if lower", func() {
		logRetention := NewBuildLogRetentionCalculator(5, 4, 0, 0).BuildLogsToRetain(makeJob(6, 0))
		Expect(logRetention.Builds).To(Equal(4))
		Expect(logRetention.Days).To(Equal(0))
	})
	It("max only set gives max", func() {
		logRetention := NewBuildLogRetentionCalculator(0, 4, 0, 0).BuildLogsToRetain(makeJob(0, 0))
		Expect(logRetention.Builds).To(Equal(4))
		Expect(logRetention.Days).To(Equal(0))
	})
	It("mix of count and days with max", func() {
		logRetention := NewBuildLogRetentionCalculator(2, 4, 3, 2).BuildLogsToRetain(makeJob(5, 5))
		Expect(logRetention.Builds).To(Equal(4))
		Expect(logRetention.Days).To(Equal(2))
	})
})

func makeJob(retainAmount int, retainAmountDays int) db.Job {
	rv := new(dbfakes.FakeJob)
	rv.ConfigReturns(atc.JobConfig{
		BuildLogRetention: &atc.BuildLogRetention{
			Builds: retainAmount,
			Days:   retainAmountDays,
		},
	})
	return rv
}
