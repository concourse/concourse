package gc_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/gc"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PipelineCollector", func() {
	var (
		pipelineCollector   Collector
		fakePipelineFactory *dbfakes.FakePipelineFactory
		logger *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakePipelineFactory = new(dbfakes.FakePipelineFactory)
		logger = lagertest.NewTestLogger("pipeline-collector-test")
	})

	JustBeforeEach(func() {
		pipelineCollector = NewPipelineCollector(
			fakePipelineFactory,
			5*time.Minute,
		)
	})

	Context("when there is a paused pipelines with builds within ttl", func() {
		var fakePipeline *dbfakes.FakePipeline

		BeforeEach(func() {
			fakePipeline = new(dbfakes.FakePipeline)
			fakePipeline.IDReturns(42)
			fakePipeline.BuildsWithTimeReturns([]db.Build{new(dbfakes.FakeBuild)}, db.Pagination{}, nil)
			fakePipeline.PausedReturns(true)

			fakePipelineFactory.AllPipelinesReturns([]db.Pipeline{fakePipeline}, nil)
		})

		It("should be destroyed", func() {
			err := pipelineCollector.Collect(logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(fakePipeline.DestroyCallCount()).To(Equal(0))
			Expect(fakePipeline.PauseCallCount()).To(Equal(0))
		})
	})

	Context("when there is a paused pipelines without builds within ttl", func() {
		var fakePipeline *dbfakes.FakePipeline

		BeforeEach(func() {
			fakePipeline = new(dbfakes.FakePipeline)
			fakePipeline.IDReturns(42)
			fakePipeline.BuildsWithTimeReturns([]db.Build{}, db.Pagination{}, nil)
			fakePipeline.PausedReturns(true)

			fakePipelineFactory.AllPipelinesReturns([]db.Pipeline{fakePipeline}, nil)
		})

		It("should be destroyed", func() {
			err := pipelineCollector.Collect(logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(fakePipeline.DestroyCallCount()).To(Equal(1))
			Expect(fakePipeline.PauseCallCount()).To(Equal(0))
		})
	})

	Context("when there is a running pipelines with builds within ttl", func() {
		var fakePipeline *dbfakes.FakePipeline

		BeforeEach(func() {
			fakePipeline = new(dbfakes.FakePipeline)
			fakePipeline.IDReturns(42)
			fakePipeline.BuildsWithTimeReturns([]db.Build{new(dbfakes.FakeBuild)}, db.Pagination{}, nil)
			fakePipeline.PausedReturns(false)

			fakePipelineFactory.AllPipelinesReturns([]db.Pipeline{fakePipeline}, nil)
		})

		It("should be destroyed", func() {
			err := pipelineCollector.Collect(logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(fakePipeline.DestroyCallCount()).To(Equal(0))
			Expect(fakePipeline.PauseCallCount()).To(Equal(0))
		})
	})

	Context("when there is a running pipelines without builds within ttl", func() {
		var fakePipeline *dbfakes.FakePipeline

		BeforeEach(func() {
			fakePipeline = new(dbfakes.FakePipeline)
			fakePipeline.IDReturns(42)
			fakePipeline.BuildsWithTimeReturns([]db.Build{}, db.Pagination{}, nil)
			fakePipeline.PausedReturns(false)

			fakePipelineFactory.AllPipelinesReturns([]db.Pipeline{fakePipeline}, nil)
		})

		It("should be destroyed", func() {
			err := pipelineCollector.Collect(logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(fakePipeline.DestroyCallCount()).To(Equal(0))
			Expect(fakePipeline.PauseCallCount()).To(Equal(1))
		})
	})

	Context("ttl is 0", func() {
		var fakePipeline *dbfakes.FakePipeline

		BeforeEach(func() {
			fakePipeline = new(dbfakes.FakePipeline)
			fakePipeline.IDReturns(42)
			fakePipeline.BuildsWithTimeReturns([]db.Build{}, db.Pagination{}, nil)
			fakePipeline.PausedReturns(false)

			fakePipelineFactory.AllPipelinesReturns([]db.Pipeline{fakePipeline}, nil)
		})

		JustBeforeEach(func() {
			pipelineCollector = NewPipelineCollector(
				fakePipelineFactory,
				0,
			)
		})

		It("should do nothing", func() {
			err := pipelineCollector.Collect(logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(fakePipeline.DestroyCallCount()).To(Equal(0))
			Expect(fakePipeline.PauseCallCount()).To(Equal(0))
		})
	})
})
