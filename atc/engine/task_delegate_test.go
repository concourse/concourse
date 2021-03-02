package engine

import (
	"encoding/json"
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
