package gc_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	. "github.com/concourse/atc/gc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildLogRetentionCalculator", func() {
	It("nothing set gives all", func() {
		Expect(NewBuildLogRetentionCalculator(0, 0).BuildLogsToRetain(makeJob(0))).To(Equal(0))
	})
	It("nothing set but job gives job", func() {
		Expect(NewBuildLogRetentionCalculator(0, 0).BuildLogsToRetain(makeJob(3))).To(Equal(3))
	})
	It("default set gives default", func() {
		Expect(NewBuildLogRetentionCalculator(5, 0).BuildLogsToRetain(makeJob(0))).To(Equal(5))
	})
	It("default and job set gives job", func() {
		Expect(NewBuildLogRetentionCalculator(5, 0).BuildLogsToRetain(makeJob(6))).To(Equal(6))
	})
	It("default and job set and max set gives max if lower", func() {
		Expect(NewBuildLogRetentionCalculator(5, 4).BuildLogsToRetain(makeJob(6))).To(Equal(4))
	})
	It("max only set gives max", func() {
		Expect(NewBuildLogRetentionCalculator(0, 4).BuildLogsToRetain(makeJob(0))).To(Equal(4))
	})
})

func makeJob(retainAmount int) db.Job {
	rv := new(dbfakes.FakeJob)
	rv.ConfigReturns(atc.JobConfig{
		BuildLogsToRetain: retainAmount,
	})
	return rv
}
