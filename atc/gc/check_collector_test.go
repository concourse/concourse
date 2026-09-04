package gc_test

import (
	"context"

	"github.com/concourse/concourse/v8/atc/db/dbfakes"
	"github.com/concourse/concourse/v8/atc/gc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CheckCollector", func() {
	var collector GcCollector
	var fakeLifecycle *dbfakes.FakeCheckLifecycle

	BeforeEach(func() {
		fakeLifecycle = new(dbfakes.FakeCheckLifecycle)

		collector = gc.NewChecksCollector(fakeLifecycle)
	})

	Describe("Run", func() {
		It("tells the check lifecycle to remove completed checks", func() {
			err := collector.Run(context.Background())
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLifecycle.DeleteCompletedChecksCallCount()).To(Equal(1))
		})
	})
})
