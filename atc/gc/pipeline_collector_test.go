package gc_test

import (
	"context"
	"time"

	"github.com/concourse/concourse/v8/atc/db/dbfakes"
	"github.com/concourse/concourse/v8/atc/gc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PipelineCollector", func() {
	var collector GcCollector
	var fakePipelineLifecycle *dbfakes.FakePipelineLifecycle

	BeforeEach(func() {
		fakePipelineLifecycle = new(dbfakes.FakePipelineLifecycle)

		collector = gc.NewPipelineCollector(fakePipelineLifecycle, time.Hour)
	})

	Describe("Run", func() {
		It("tells the pipeline lifecycle to remove abandoned pipelines", func() {
			err := collector.Run(context.TODO())
			Expect(err).NotTo(HaveOccurred())

			Expect(fakePipelineLifecycle.ArchiveAbandonedPipelinesCallCount()).To(Equal(1))
		})

		It("tells the pipeline lifecycle to remove archived pipelines", func() {
			err := collector.Run(context.TODO())
			Expect(err).NotTo(HaveOccurred())

			Expect(fakePipelineLifecycle.DestroyArchivedPipelinesCallCount()).To(Equal(1))
			Expect(fakePipelineLifecycle.DestroyArchivedPipelinesArgsForCall(0)).To(Equal(time.Hour))
		})
	})
})
