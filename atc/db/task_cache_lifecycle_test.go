package db_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbtest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TaskCacheLifecycle", func() {
	var taskCacheLifecycle db.TaskCacheLifecycle

	BeforeEach(func() {
		taskCacheLifecycle = db.NewTaskCacheLifecycle(dbConn)
	})

	It("cleans up task caches belonging to an archived pipeline", func() {
		archivedScenario := dbtest.Setup(
			builder.WithPipeline(atc.Config{
				Jobs: []atc.JobConfig{
					{Name: "some-job"},
				},
			}),
		)
		otherScenario := dbtest.Setup(
			builder.WithPipeline(atc.Config{
				Jobs: []atc.JobConfig{
					{Name: "some-other-job"},
				},
			}),
		)
		taskCache, err := taskCacheFactory.FindOrCreate(archivedScenario.Job("some-job").ID(), "some-step", "some-path")
		Expect(err).ToNot(HaveOccurred())

		_, err = taskCacheFactory.FindOrCreate(otherScenario.Job("some-other-job").ID(), "some-step", "some-path")
		Expect(err).ToNot(HaveOccurred())

		err = archivedScenario.Pipeline.Archive()
		Expect(err).ToNot(HaveOccurred())

		deletedCacheIDs, err := taskCacheLifecycle.CleanUpInvalidTaskCaches()
		Expect(err).ToNot(HaveOccurred())
		Expect(deletedCacheIDs).To(ConsistOf(taskCache.ID()))
	})

	It("cleans up task caches belonging to a paused pipeline", func() {
		archivedScenario := dbtest.Setup(
			builder.WithPipeline(atc.Config{
				Jobs: []atc.JobConfig{
					{Name: "some-job"},
				},
			}),
		)
		otherScenario := dbtest.Setup(
			builder.WithPipeline(atc.Config{
				Jobs: []atc.JobConfig{
					{Name: "some-other-job"},
				},
			}),
		)
		taskCache, err := taskCacheFactory.FindOrCreate(archivedScenario.Job("some-job").ID(), "some-step", "some-path")
		Expect(err).ToNot(HaveOccurred())

		_, err = taskCacheFactory.FindOrCreate(otherScenario.Job("some-other-job").ID(), "some-step", "some-path")
		Expect(err).ToNot(HaveOccurred())

		err = archivedScenario.Pipeline.Pause("tester")
		Expect(err).ToNot(HaveOccurred())

		deletedCacheIDs, err := taskCacheLifecycle.CleanUpInvalidTaskCaches()
		Expect(err).ToNot(HaveOccurred())
		Expect(deletedCacheIDs).To(ConsistOf(taskCache.ID()))
	})
})
