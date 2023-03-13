package db_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbtest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TaskCacheLifecycle", func() {
	var taskCacheLifecycle db.TaskCacheLifecycle
	var plan atc.Plan

	BeforeEach(func() {
		taskCacheLifecycle = db.NewTaskCacheLifecycle(dbConn)
		plan = atc.Plan{
			Get: &atc.GetPlan{
				Name: "some-name",
			},
		}
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

	It("cleans up task caches belonging to a paused pipeline if its jobs are not running", func() {
		var pendingBuild, startedBuild, finishedBuild db.Build

		pausedScenario := dbtest.Setup(
			builder.WithPipeline(atc.Config{
				Jobs: []atc.JobConfig{
					{Name: "some-job-1"},
					{Name: "some-job-2"},
				},
			}),
			builder.WithPendingJobBuild(&pendingBuild, "some-job-1"),
			builder.WithStartedJobBuild(&startedBuild, "some-job-2", plan),
		)
		otherScenario := dbtest.Setup(
			builder.WithPipeline(atc.Config{
				Jobs: []atc.JobConfig{
					{Name: "some-other-job"},
				},
			}),
			builder.WithPendingJobBuild(&finishedBuild, "some-other-job"),
		)

		err := finishedBuild.Finish(db.BuildStatusSucceeded)
		Expect(err).ToNot(HaveOccurred())

		taskCache, err := taskCacheFactory.FindOrCreate(otherScenario.Job("some-other-job").ID(), "some-step", "some-path")
		Expect(err).ToNot(HaveOccurred())

		_, err = taskCacheFactory.FindOrCreate(pausedScenario.Job("some-job-1").ID(), "some-step", "some-path")
		Expect(err).ToNot(HaveOccurred())

		_, err = taskCacheFactory.FindOrCreate(pausedScenario.Job("some-job-2").ID(), "some-step", "some-path")
		Expect(err).ToNot(HaveOccurred())

		err = pausedScenario.Pipeline.Pause("tester")
		Expect(err).ToNot(HaveOccurred())

		err = otherScenario.Pipeline.Pause("tester")
		Expect(err).ToNot(HaveOccurred())

		deletedCacheIDs, err := taskCacheLifecycle.CleanUpInvalidTaskCaches()
		Expect(err).ToNot(HaveOccurred())
		Expect(deletedCacheIDs).To(ConsistOf(taskCache.ID()))
	})

	It("cleans up task caches belonging to a paused job if it is not running", func() {
		var startedBuild, finishedBuild db.Build

		pausedScenario := dbtest.Setup(
			builder.WithPipeline(atc.Config{
				Jobs: []atc.JobConfig{
					{Name: "some-job-1"},
					{Name: "some-job-2"},
				},
			}),
			builder.WithPendingJobBuild(&finishedBuild, "some-job-1"),
			builder.WithStartedJobBuild(&startedBuild, "some-job-2", plan),
		)

		err := finishedBuild.Finish(db.BuildStatusSucceeded)
		Expect(err).ToNot(HaveOccurred())

		err = pausedScenario.Job("some-job-1").Pause("tester")
		Expect(err).ToNot(HaveOccurred())

		err = pausedScenario.Job("some-job-2").Pause("tester")
		Expect(err).ToNot(HaveOccurred())

		taskCache, err := taskCacheFactory.FindOrCreate(pausedScenario.Job("some-job-1").ID(), "some-step", "some-path")
		Expect(err).ToNot(HaveOccurred())

		_, err = taskCacheFactory.FindOrCreate(pausedScenario.Job("some-job-2").ID(), "some-step", "some-path")
		Expect(err).ToNot(HaveOccurred())

		deletedCacheIDs, err := taskCacheLifecycle.CleanUpInvalidTaskCaches()
		Expect(err).ToNot(HaveOccurred())
		Expect(deletedCacheIDs).To(ConsistOf(taskCache.ID()))
	})
})
