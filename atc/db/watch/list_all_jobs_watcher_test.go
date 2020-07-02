package watch_test

import (
	"context"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
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

			access        *accessorfakes.FakeAccess
			mtx           sync.Mutex
			invokedEvents []watch.DashboardJobEvent
		)

		PutEvent := func(job db.Job) watch.DashboardJobEvent {
			return watch.DashboardJobEvent{
				ID:   job.ID(),
				Type: watch.Put,
				Job:  toDashboardJob(job),
			}
		}

		DeleteEvent := func(jobID int) watch.DashboardJobEvent {
			return watch.DashboardJobEvent{
				ID:   jobID,
				Type: watch.Delete,
			}
		}

		getInvokedEvents := func() []watch.DashboardJobEvent {
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

			pipeline, _, err = team.SavePipeline("pipeline", atc.Config{
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

			access = new(accessorfakes.FakeAccess)
			access.IsAdminReturns(true)

			resetInvokedEvents()
		})

		JustBeforeEach(func() {
			eventsChan := watcher.WatchListAllJobs(ctx, access)
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
			team.SavePipeline("pipeline", atc.Config{
				Jobs: atc.JobConfigs{{Name: "job2"}},
			}, pipeline.ConfigVersion(), false)

			Eventually(getInvokedEvents).Should(ConsistOf(DeleteEvent(job1.ID()), PutEvent(job2)))
		})

		It("notifies on job reactivated", func() {
			pipeline, _, _ = team.SavePipeline("pipeline", atc.Config{
				Jobs: atc.JobConfigs{{Name: "job2"}},
			}, pipeline.ConfigVersion(), false)

			Eventually(getInvokedEvents).Should(HaveLen(2))
			resetInvokedEvents()

			team.SavePipeline("pipeline", atc.Config{
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

		Describe("access control for non-admins", func() {
			BeforeEach(func() {
				access.IsAdminReturns(false)
			})

			Context("when the user has access to the team", func() {
				BeforeEach(func() {
					access.TeamNamesReturns([]string{"team"})
				})

				It("forwards the notification", func() {
					job1.CreateBuild()
					Eventually(getInvokedEvents).Should(ContainElement(PutEvent(job1)))
				})
			})

			Context("when the user does not have access to the team", func() {
				BeforeEach(func() {
					access.TeamNamesReturns([]string{"other-team"})
				})

				It("does not forward the notification", func() {
					job1.CreateBuild()
					Consistently(getInvokedEvents).Should(BeEmpty())
				})

				Context("when the notification is a delete", func() {
					It("forwards the notification", func() {
						pipeline.Destroy()
						Eventually(getInvokedEvents).Should(ContainElement(DeleteEvent(job1.ID())))
					})
				})

				Context("when the pipeline is public", func() {
					BeforeEach(func() {
						pipeline.Expose()
					})

					It("forwards the notification", func() {
						job1.CreateBuild()
						Eventually(getInvokedEvents).Should(ContainElement(PutEvent(job1)))
					})
				})
			})
		})

		It("subscribers don't block the watcher if their events channel isn't drained", func() {
			watcher.WatchListAllJobs(ctx, access)

			for i := 0; i < 100; i++ {
				job1.CreateBuild()
			}

			Eventually(getInvokedEvents).Should(HaveLen(100))
		})
	})

	It("cancelling the context halts the subscriber", func() {
		time.AfterFunc(50*time.Millisecond, cancel)
		eventsChan := watcher.WatchListAllJobs(ctx, new(accessorfakes.FakeAccess))
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

func toDashboardJob(job db.Job) *atc.DashboardJob {
	job.Reload()

	finishedBuild, nextBuild, err := job.FinishedAndNextBuild()
	Expect(err).ToNot(HaveOccurred())

	pipeline, _, err := job.Pipeline()
	Expect(err).ToNot(HaveOccurred())

	return &atc.DashboardJob{
		ID:              job.ID(),
		Name:            job.Name(),
		PipelineName:    job.PipelineName(),
		PipelinePublic:  pipeline.Public(),
		TeamName:        job.TeamName(),
		Paused:          job.Paused(),
		HasNewInputs:    job.HasNewInputs(),
		FinishedBuild:   toDashboardBuild(finishedBuild),
		TransitionBuild: toDashboardBuild(finishedBuild),
		NextBuild:       toDashboardBuild(nextBuild),
	}
}

func toDashboardBuild(build db.Build) *atc.DashboardBuild {
	if build == nil {
		return nil
	}
	return &atc.DashboardBuild{
		ID:           build.ID(),
		Name:         build.Name(),
		JobName:      build.JobName(),
		PipelineName: build.PipelineName(),
		TeamName:     build.TeamName(),
		Status:       string(build.Status()),
		StartTime:    build.StartTime(),
		EndTime:      build.EndTime(),
	}
}
