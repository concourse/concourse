package builder_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/engine/builder"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/vars"
)

var _ = Describe("SetPipelineStepDelegate", func() {
	var (
		logger    *lagertest.TestLogger
		fakeBuild *dbfakes.FakeBuild
		fakeClock *fakeclock.FakeClock
		buildVars *vars.BuildVariables

		now      = time.Date(1991, 6, 3, 5, 30, 0, 0, time.UTC)
		delegate exec.SetPipelineStepDelegate
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

		delegate = builder.NewSetPipelineStepDelegate(fakeBuild, "some-plan-id", buildVars, fakeClock)
	})

	Describe("SetPipelineChanged", func() {
		JustBeforeEach(func() {
			delegate.SetPipelineChanged(logger, true)
		})

		It("saves an event", func() {
			Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
			Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.SetPipelineChanged{
				Origin:  event.Origin{ID: event.OriginID("some-plan-id")},
				Changed: true,
			}))
		})
	})
})
