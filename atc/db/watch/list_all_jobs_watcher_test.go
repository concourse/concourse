package watch_test

import (
	"context"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/watch"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ListAllJobsWatcher", func() {
	var (
		watcher *watch.ListAllJobsWatcher
		err     error

		dummyLogger lager.Logger
		ctx         context.Context
		cancel      context.CancelFunc
	)

	BeforeEach(func() {
		dummyLogger = lagertest.NewTestLogger("test")
		ctx, cancel = context.WithCancel(context.Background())
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		watcher, err = watch.NewListAllJobsWatcher(dummyLogger, dbConn, lockFactory)
	})

	Describe("trigger initialization", func() {
		It("trigger on jobs INSERT, UPDATE, and DELETE", func() {
			Expect(triggerOperations("jobs_notify")).To(ConsistOf("INSERT", "UPDATE", "DELETE"))
		})

		It("trigger on pipelines UPDATE", func() {
			Expect(triggerOperations("pipelines_notify")).To(ConsistOf("UPDATE"))
		})

		It("trigger on teams UPDATE", func() {
			Expect(triggerOperations("teams_notify")).To(ConsistOf("UPDATE"))
		})

		Context("when the lock is already acquired", func() {
			var triggerLock lock.Lock

			BeforeEach(func() {
				clearNotifyTriggers()

				triggerLock, _, _ = lockFactory.Acquire(dummyLogger, lock.NewCreateWatchTriggersLockID())
			})

			AfterEach(func() {
				triggerLock.Release()
			})

			It("does not create any triggers", func() {
				Expect(triggerOperations("jobs_notify")).To(BeEmpty())
			})

			It("does not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("notifications", func() {
		var (
			team     db.Team
			pipeline db.Pipeline
			job1     db.Job
			job2     db.Job

			mtx           sync.Mutex
			invokedEvents []watch.JobSummaryEvent
		)

		PutEvent := func(job db.Job) watch.JobSummaryEvent {
			return watch.JobSummaryEvent{
				ID:   job.ID(),
				Type: watch.Put,
				Job:  toJobSummary(job),
			}
		}

		DeleteEvent := func(jobID int) watch.JobSummaryEvent {
			return watch.JobSummaryEvent{
				ID:   jobID,
				Type: watch.Delete,
			}
		}

		getInvokedEvents := func() []watch.JobSummaryEvent {
			mtx.Lock()
			defer mtx.Unlock()
			return invokedEvents
		}

		resetInvokedEvents := func() {
			mtx.Lock()
			defer mtx.Unlock()
			invokedEvents = invokedEvents[:0]
		}

		BeforeEach(func() {
			var err error
			teamFactory := db.NewTeamFactory(dbConn, lockFactory)
			team, err = teamFactory.CreateTeam(atc.Team{ID: 1, Name: "team"})
			Expect(err).ToNot(HaveOccurred())

			pipeline, _, err = team.SavePipeline(atc.PipelineRef{Name: "pipeline"}, atc.Config{
				Jobs: atc.JobConfigs{{Name: "job1"}, {Name: "job2"}},
			}, 0, false)
			Expect(err).ToNot(HaveOccurred())

			var found bool
			job1, found, err = pipeline.Job("job1")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			job2, found, err = pipeline.Job("job2")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resetInvokedEvents()
		})

		JustBeforeEach(func() {
			eventsChan, _ := watcher.WatchListAllJobs(ctx)
			go func() {
				for events := range eventsChan {
					mtx.Lock()
					invokedEvents = append(invokedEvents, events...)
					mtx.Unlock()
				}
			}()
		})

		It("notifies on build creation", func() {
			job1.CreateBuild()
			Eventually(getInvokedEvents).Should(ConsistOf(PutEvent(job1)))
		})

		It("notifies on build completion", func() {
			build, _ := job1.CreateBuild()
			Eventually(getInvokedEvents).Should(HaveLen(1))
			resetInvokedEvents()

			build.Finish(db.BuildStatusSucceeded)
			Eventually(getInvokedEvents).Should(ConsistOf(PutEvent(job1)))
		})

		It("notifies on pipeline renamed", func() {
			pipeline.Rename("blah")
			Eventually(getInvokedEvents).Should(ConsistOf(PutEvent(job1), PutEvent(job2)))
		})

		It("notifies on team renamed", func() {
			team.Rename("blah")
			Eventually(getInvokedEvents).Should(ConsistOf(PutEvent(job1), PutEvent(job2)))
		})

		It("notifies on team renamed", func() {
			team.Rename("blah")
			Eventually(getInvokedEvents).Should(ConsistOf(PutEvent(job1), PutEvent(job2)))
		})

		It("notifies on job deactivated", func() {
			team.SavePipeline(atc.PipelineRef{Name: "pipeline"}, atc.Config{
				Jobs: atc.JobConfigs{{Name: "job2"}},
			}, pipeline.ConfigVersion(), false)

			Eventually(getInvokedEvents).Should(ConsistOf(DeleteEvent(job1.ID()), PutEvent(job2)))
		})

		It("notifies on job reactivated", func() {
			pipeline, _, _ = team.SavePipeline(atc.PipelineRef{Name: "pipeline"}, atc.Config{
				Jobs: atc.JobConfigs{{Name: "job2"}},
			}, pipeline.ConfigVersion(), false)

			Eventually(getInvokedEvents).Should(HaveLen(2))
			resetInvokedEvents()

			team.SavePipeline(atc.PipelineRef{Name: "pipeline"}, atc.Config{
				Jobs: atc.JobConfigs{{Name: "job1"}, {Name: "job2"}},
			}, pipeline.ConfigVersion(), false)
			Eventually(getInvokedEvents).Should(ConsistOf(PutEvent(job1), PutEvent(job2)))
		})

		It("notifies on job deleted", func() {
			pipeline.Destroy()

			Eventually(getInvokedEvents).Should(ConsistOf(DeleteEvent(job1.ID()), DeleteEvent(job2.ID())))
		})

		It("notifies on pipeline exposed", func() {
			pipeline.Expose()

			Eventually(getInvokedEvents).Should(ConsistOf(PutEvent(job1), PutEvent(job2)))
		})

		It("notifies on pipeline hidden", func() {
			pipeline.Hide()

			Eventually(getInvokedEvents).Should(ConsistOf(PutEvent(job1), PutEvent(job2)))
		})

		It("subscribers don't block the watcher if their events channel isn't drained", func() {
			watcher.WatchListAllJobs(ctx)

			for i := 0; i < 100; i++ {
				job1.CreateBuild()
			}

			Eventually(getInvokedEvents).Should(HaveLen(100))
		})
	})

	It("cancelling the context halts the subscriber", func() {
		time.AfterFunc(50*time.Millisecond, cancel)
		eventsChan, _ := watcher.WatchListAllJobs(ctx)
		Eventually(eventsChan).Should(BeClosed())
	})
})

func triggerOperations(triggerName string) []string {
	rows, err := dbConn.Query(
		`SELECT event_manipulation FROM information_schema.triggers WHERE trigger_name = '` + triggerName + `'`,
	)
	Expect(err).ToNot(HaveOccurred())
	var operations []string
	for rows.Next() {
		var op string
		err = rows.Scan(&op)
		Expect(err).ToNot(HaveOccurred())
		operations = append(operations, op)
	}
	return operations
}

func clearNotifyTriggers() {
	rows, err := dbConn.Query(
		`SELECT trigger_name, event_object_table
		 FROM information_schema.triggers
		 WHERE trigger_name LIKE '%_notify'
		 GROUP BY 1, 2`,
	)
	Expect(err).ToNot(HaveOccurred())
	type trigger struct {
		name      string
		tableName string
	}
	var triggers []trigger
	for rows.Next() {
		var t trigger
		err = rows.Scan(&t.name, &t.tableName)
		Expect(err).ToNot(HaveOccurred())
		triggers = append(triggers, t)
	}
	for _, t := range triggers {
		_, err = dbConn.Exec(`DROP TRIGGER ` + t.name + ` ON ` + t.tableName)
		Expect(err).ToNot(HaveOccurred())
	}
}

func toJobSummary(job db.Job) *atc.JobSummary {
	job.Reload()

	finishedBuild, nextBuild, err := job.FinishedAndNextBuild()
	Expect(err).ToNot(HaveOccurred())

	pipeline, _, err := job.Pipeline()
	Expect(err).ToNot(HaveOccurred())

	return &atc.JobSummary{
		ID:              job.ID(),
		Name:            job.Name(),
		PipelineID:      job.PipelineID(),
		PipelineName:    job.PipelineName(),
		PipelinePublic:  pipeline.Public(),
		TeamName:        job.TeamName(),
		Paused:          job.Paused(),
		HasNewInputs:    job.HasNewInputs(),
		FinishedBuild:   toBuildSummary(finishedBuild),
		TransitionBuild: toBuildSummary(finishedBuild),
		NextBuild:       toBuildSummary(nextBuild),
	}
}

func toBuildSummary(build db.Build) *atc.BuildSummary {
	if build == nil {
		return nil
	}
	return &atc.BuildSummary{
		ID:           build.ID(),
		Name:         build.Name(),
		JobName:      build.JobName(),
		PipelineName: build.PipelineName(),
		PipelineID:   build.PipelineID(),
		TeamName:     build.TeamName(),
		Status:       atc.BuildStatus(build.Status()),
		StartTime:    build.StartTime().Unix(),
		EndTime:      build.EndTime().Unix(),
	}
}
