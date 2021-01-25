package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/policy/policyfakes"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/concourse/concourse/vars"
)

var noopStepper exec.Stepper = func(atc.Plan) exec.Step {
	Fail("cannot create substep")
	return nil
}

var _ = Describe("TaskDelegate", func() {
	var (
		logger              *lagertest.TestLogger
		fakeBuild           *dbfakes.FakeBuild
		fakeClock           *fakeclock.FakeClock
		fakePolicyChecker   *policyfakes.FakeChecker
		fakeArtifactSourcer *workerfakes.FakeArtifactSourcer
		fakeWorkerFactory   *dbfakes.FakeWorkerFactory
		fakeLockFactory     *lockfakes.FakeLockFactory

		state exec.RunState

		now = time.Date(1991, 6, 3, 5, 30, 0, 0, time.UTC)

		delegate *taskDelegate

		exitStatus exec.ExitStatus
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeBuild = new(dbfakes.FakeBuild)
		fakeClock = fakeclock.NewFakeClock(now)
		credVars := vars.StaticVariables{
			"source-param": "super-secret-source",
			"git-key":      "{\n123\n456\n789\n}\n",
		}
		state = exec.NewRunState(noopStepper, credVars, true)

		fakePolicyChecker = new(policyfakes.FakeChecker)
		fakeArtifactSourcer = new(workerfakes.FakeArtifactSourcer)
		fakeWorkerFactory = new(dbfakes.FakeWorkerFactory)
		fakeLockFactory = new(lockfakes.FakeLockFactory)

		delegate = NewTaskDelegate(fakeBuild, "some-plan-id", state, fakeClock, fakePolicyChecker, fakeArtifactSourcer, fakeWorkerFactory, fakeLockFactory).(*taskDelegate)

		delegate.SetTaskConfig(atc.TaskConfig{
			Platform: "some-platform",
			Run: atc.TaskRunConfig{
				Path: "some-foo-path",
				Dir:  "some-bar-dir",
			},
		})
	})

	Describe("Initializing", func() {
		JustBeforeEach(func() {
			delegate.Initializing(logger)
		})

		It("saves an event", func() {
			Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
			event := fakeBuild.SaveEventArgsForCall(0)
			Expect(event.EventType()).To(Equal(atc.EventType("initialize-task")))
		})

		It("calls SaveEvent with the taskConfig", func() {
			Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
			event := fakeBuild.SaveEventArgsForCall(0)
			Expect(json.Marshal(event)).To(MatchJSON(`{
				"time": 675927000,
				"origin": {"id": "some-plan-id"},
				"config": {
					"platform": "some-platform",
					"image":"",
					"run": {
						"path": "some-foo-path",
						"args": null,
						"dir": "some-bar-dir"
					},
					"inputs":null
				}
			}`))
		})
	})

	Describe("Starting", func() {
		JustBeforeEach(func() {
			delegate.Starting(logger)
		})

		It("saves an event", func() {
			Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
			event := fakeBuild.SaveEventArgsForCall(0)
			Expect(event.EventType()).To(Equal(atc.EventType("start-task")))
		})

		It("calls SaveEvent with the taskConfig", func() {
			Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
			event := fakeBuild.SaveEventArgsForCall(0)
			Expect(json.Marshal(event)).To(MatchJSON(`{
				"time": 675927000,
				"origin": {"id": "some-plan-id"},
				"config": {
					"platform": "some-platform",
					"image":"",
					"run": {
						"path": "some-foo-path",
						"args": null,
						"dir": "some-bar-dir"
					},
					"inputs":null
				}
			}`))
		})
	})

	Describe("Finished", func() {
		var fakeClient *workerfakes.FakeClient
		var fakeStrategy *workerfakes.FakeContainerPlacementStrategy

		BeforeEach(func() {
			fakeClient = new(workerfakes.FakeClient)
			fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)
		})

		JustBeforeEach(func() {
			delegate.Finished(logger, exitStatus, fakeStrategy, fakeClient)
		})

		It("saves an event", func() {
			Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
			event := fakeBuild.SaveEventArgsForCall(0)
			Expect(event.EventType()).To(Equal(atc.EventType("finish-task")))
		})

		Context("with the limit active tasks strategy", func() {
			var fakeWorker *dbfakes.FakeWorker

			BeforeEach(func() {
				fakeStrategy.ModifiesActiveTasksReturns(true)

				fakeWorker = workerStub()
				fakeWorker.IncreaseActiveTasks()
				fakeWorkerFactory.GetWorkerReturns(fakeWorker, true, nil)
			})

			It("decreases the active tasks", func() {
				Expect(fakeWorker.ActiveTasks()).To(Equal(0))
			})
		})
	})

	Describe("SelectWorker", func() {
		var (
			fakePool      *workerfakes.FakePool
			fakeClient    *workerfakes.FakeClient
			fakeWorker    *dbfakes.FakeWorker
			fakeStrategy  *workerfakes.FakeContainerPlacementStrategy
			fakeLock      *lockfakes.FakeLock
			owner         db.ContainerOwner
			containerSpec worker.ContainerSpec
			workerSpec    worker.WorkerSpec

			chosenWorker worker.Client
			err          error
		)

		BeforeEach(func() {
			fakePool = new(workerfakes.FakePool)
			fakeClient = new(workerfakes.FakeClient)
			owner = containerOwnerDummy()
			containerSpec = containerSpecDummy()
			workerSpec = workerSpecDummy()

			fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)

			fakeLock = new(lockfakes.FakeLock)
			fakeLockFactory.AcquireReturns(fakeLock, true, nil)

			fakeWorker = workerStub()
			fakeWorkerFactory.GetWorkerReturns(fakeWorker, true, nil)
		})

		JustBeforeEach(func() {
			chosenWorker, err = delegate.SelectWorker(
				context.Background(),
				fakePool,
				owner,
				containerSpec,
				workerSpec,
				fakeStrategy,
				10*time.Millisecond,
				20*time.Millisecond,
			)
		})

		Context("when using the limit-active-tasks strategy", func() {
			BeforeEach(func() {
				fakeStrategy.ModifiesActiveTasksReturns(true)
			})

			AfterEach(func() {
				Expect(fakeLockFactory.AcquireCallCount()).To(BeNumerically(">", 0), "did not acquire a lock")
				Expect(fakeLock.ReleaseCallCount()).To(Equal(fakeLockFactory.AcquireCallCount()), "did not release the lock")
			})

			Context("when there is a worker available", func() {
				BeforeEach(func() {
					fakePool.SelectWorkerReturns(fakeClient, nil)
				})

				It("returns the chosen worker", func() {
					Expect(err).ToNot(HaveOccurred())
					Expect(chosenWorker).To(Equal(fakeClient))
				})

				Context("when the container does not yet exist", func() {
					BeforeEach(func() {
						fakePool.ContainerInWorkerReturns(false, nil)
					})

					It("increments the worker's active tasks", func() {
						Expect(fakeWorker.ActiveTasks()).To(Equal(1))
					})
				})

				Context("when the container is already present on the worker", func() {
					BeforeEach(func() {
						fakePool.ContainerInWorkerReturns(true, nil)
					})

					It("does not increment the worker's active tasks", func() {
						Expect(fakeWorker.ActiveTasks()).To(Equal(0))
					})
				})
			})

			Context("when no worker is immediately available", func() {
				var buf *bytes.Buffer

				allWorkersFullError := worker.NoWorkerFitContainerPlacementStrategyError{Strategy: "limit-active-tasks"}

				BeforeEach(func() {
					fakePool.SelectWorkerReturnsOnCall(0, nil, allWorkersFullError)
					fakePool.SelectWorkerReturnsOnCall(1, nil, allWorkersFullError)
					fakePool.SelectWorkerReturnsOnCall(2, nil, allWorkersFullError)
					fakePool.SelectWorkerReturnsOnCall(3, fakeClient, nil)

					buf = new(bytes.Buffer)
					delegate.BuildStepDelegate.(*buildStepDelegate).stdout = buf
				})

				It("returns the chosen worker", func() {
					Expect(err).ToNot(HaveOccurred())
					Expect(chosenWorker).To(Equal(fakeClient))
				})

				It("writes status to status writer", func() {
					Expect(buf.String()).To(ContainSubstring("All workers are busy at the moment"))
					Expect(buf.String()).To(ContainSubstring("Found a free worker after waiting"))
				})

				It("task waiting metrics is gauged", func() {
					labels := metric.TasksWaitingLabels{
						TeamId:     "123",
						WorkerTags: "step_tags",
						Platform:   "some-platform",
					}

					Expect(metric.Metrics.TasksWaiting).To(HaveKey(labels))

					// Verify that when one task is waiting the gauge is increased...
					Eventually(metric.Metrics.TasksWaiting[labels].Max()).Should(Equal(float64(1)))
					// and only increased once...
					Consistently(metric.Metrics.TasksWaiting[labels].Max()).Should(BeNumerically("<", 2))
					// and then decreased.
					Eventually(metric.Metrics.TasksWaiting[labels].Max()).Should(Equal(float64(0)))
				})
			})

			Context("when selecting a worker fails", func() {
				BeforeEach(func() {
					fakePool.SelectWorkerReturns(nil, errors.New("nope"))
				})

				It("returns the error", func() {
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when not using the limit-active-tasks strategy", func() {
			BeforeEach(func() {
				fakeStrategy.ModifiesActiveTasksReturns(false)
				fakePool.SelectWorkerReturns(fakeClient, nil)
			})

			It("returns the selected worker", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(chosenWorker).To(Equal(fakeClient))
			})

			It("does not acquire a lock", func() {
				Expect(fakeLockFactory.AcquireCallCount()).To(Equal(0))
			})

			Context("when no worker is immediately available", func() {
				BeforeEach(func() {
					fakePool.SelectWorkerReturns(nil, worker.NoWorkerFitContainerPlacementStrategyError{Strategy: "volume-locality"})
				})

				// this probably isn't desired behaviour, but keeping it for
				// backward compatibility until we figure out how to wait for
				// all strategies. this will probably involve adding a
				// SelectWorker to the BuildStepDelegate or something
				It("does not wait for a worker to be present", func() {
					Expect(err).To(HaveOccurred())
				})
			})

			Context("when selecting a worker fails", func() {
				BeforeEach(func() {
					fakePool.SelectWorkerReturns(nil, errors.New("nope"))
				})

				It("returns the error", func() {
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})

func containerSpecDummy() worker.ContainerSpec {
	cpu := uint64(1024)
	memory := uint64(1024)

	return worker.ContainerSpec{
		TeamID: 123,
		ImageSpec: worker.ImageSpec{
			ImageArtifactSource: new(workerfakes.FakeStreamableArtifactSource),
			Privileged:          false,
		},
		Limits: worker.ContainerLimits{
			CPU:    &cpu,
			Memory: &memory,
		},
		Dir:     "some-artifact-root",
		Env:     []string{"SECURE=secret-task-param"},
		Inputs:  []worker.InputSource{},
		Outputs: worker.OutputPaths{},
	}
}

func workerSpecDummy() worker.WorkerSpec {
	return worker.WorkerSpec{
		TeamID:   123,
		Platform: "some-platform",
		Tags:     []string{"step", "tags"},
	}
}

func containerOwnerDummy() db.ContainerOwner {
	return db.NewBuildStepContainerOwner(
		1234,
		atc.PlanID("42"),
		123,
	)
}

func workerStub() *dbfakes.FakeWorker {
	fakeWorker := new(dbfakes.FakeWorker)
	fakeWorker.NameReturns("some-worker")

	activeTasks := 0
	fakeWorker.IncreaseActiveTasksStub = func() error {
		activeTasks++
		return nil
	}
	fakeWorker.DecreaseActiveTasksStub = func() error {
		activeTasks--
		return nil
	}
	fakeWorker.ActiveTasksStub = func() (int, error) {
		return activeTasks, nil
	}
	return fakeWorker
}
