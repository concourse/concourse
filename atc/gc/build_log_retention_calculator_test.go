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
		count, days := NewBuildLogRetentionCalculator(0, 0, 0, 0).BuildLogsToRetain(makeJob(0, 0))
		Expect(count).To(Equal(0))
		Expect(days).To(Equal(0))
	})
	It("nothing set but job gives job", func() {
		count, days := NewBuildLogRetentionCalculator(0, 0,0, 0).BuildLogsToRetain(makeJob(3, 0))
		Expect(count).To(Equal(3))
		Expect(days).To(Equal(0))
	})
	It("default set gives default", func() {
		count, days := NewBuildLogRetentionCalculator(5, 0, 0, 0).BuildLogsToRetain(makeJob(0, 0))
		Expect(count).To(Equal(5))
		Expect(days).To(Equal(0))
	})
	It("default and job set gives job", func() {
		count, days := NewBuildLogRetentionCalculator(5, 0, 0, 0).BuildLogsToRetain(makeJob(6, 0))
		Expect(count).To(Equal(6))
		Expect(days).To(Equal(0))
	})
	It("default and job set and max set gives max if lower", func() {
		count, days := NewBuildLogRetentionCalculator(5, 4, 0, 0).BuildLogsToRetain(makeJob(6, 0))
		Expect(count).To(Equal(4))
		Expect(days).To(Equal(0))
	})
	It("max only set gives max", func() {
		count, days := NewBuildLogRetentionCalculator(0, 4,0, 0).BuildLogsToRetain(makeJob(0, 0))
		Expect(count).To(Equal(4))
		Expect(days).To(Equal(0))
	})
	It("mix of count and days with max", func() {
		count, days := NewBuildLogRetentionCalculator(2, 4,3, 2).BuildLogsToRetain(makeJob(5, 5))
		Expect(count).To(Equal(4))
		Expect(days).To(Equal(2))
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
