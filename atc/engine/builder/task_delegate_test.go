package builder_test

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/engine/builder"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/vars"
)

var _ = Describe("TaskDelegate", func() {
	var (
		logger    *lagertest.TestLogger
		fakeBuild *dbfakes.FakeBuild
		fakeClock *fakeclock.FakeClock
		buildVars *vars.BuildVariables

		now = time.Date(1991, 6, 3, 5, 30, 0, 0, time.UTC)

		delegate exec.TaskDelegate

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
		buildVars = vars.NewBuildVariables(credVars, true)

		delegate = builder.NewTaskDelegate(fakeBuild, "some-plan-id", buildVars, fakeClock)
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
			b := `{"time":.*,"origin":{"id":"some-plan-id"},"config":{"platform":"some-platform","image":"","run":{"path":"some-foo-path","args":null,"dir":"some-bar-dir"},"inputs":null}}`
			Expect(json.Marshal(event)).To(MatchRegexp(b))
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
			b := `{"time":.*,"origin":{"id":"some-plan-id"},"config":{"platform":"some-platform","image":"","run":{"path":"some-foo-path","args":null,"dir":"some-bar-dir"},"inputs":null}}`
			Expect(json.Marshal(event)).To(MatchRegexp(b))
		})
	})

	Describe("Finished", func() {
		JustBeforeEach(func() {
			delegate.Finished(logger, exitStatus)
		})

		It("saves an event", func() {
			Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
			event := fakeBuild.SaveEventArgsForCall(0)
			Expect(event.EventType()).To(Equal(atc.EventType("finish-task")))
		})
	})
})
