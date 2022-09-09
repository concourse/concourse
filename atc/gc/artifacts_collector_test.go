package gc_test

import (
	"context"

	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/gc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ArtifactCollector", func() {
	var collector GcCollector
	var fakeArtifactLifecycle *dbfakes.FakeWorkerArtifactLifecycle

	BeforeEach(func() {
		fakeArtifactLifecycle = new(dbfakes.FakeWorkerArtifactLifecycle)

		collector = gc.NewArtifactCollector(fakeArtifactLifecycle)
	})

	Describe("Run", func() {
		It("tells the artifact lifecycle to remove expired artifacts", func() {
			err := collector.Run(context.TODO())
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeArtifactLifecycle.RemoveExpiredArtifactsCallCount()).To(Equal(1))
		})
	})
})
