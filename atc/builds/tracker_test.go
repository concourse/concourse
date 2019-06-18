package builds_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc/builds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/engine/enginefakes"
)

var _ = Describe("Tracker", func() {
	var (
		fakeBuildFactory *dbfakes.FakeBuildFactory
		fakeEngine       *enginefakes.FakeEngine

		tracker *builds.Tracker
		logger  *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakeBuildFactory = new(dbfakes.FakeBuildFactory)
		fakeEngine = new(enginefakes.FakeEngine)

		logger = lagertest.NewTestLogger("test")

		tracker = builds.NewTracker(
			logger,
			fakeBuildFactory,
			fakeEngine,
		)
	})

	Describe("Track", func() {
		var inFlightBuilds []*dbfakes.FakeBuild
		var engineBuilds []*enginefakes.FakeRunnable

		BeforeEach(func() {
			inFlightBuilds = []*dbfakes.FakeBuild{
				new(dbfakes.FakeBuild),
				new(dbfakes.FakeBuild),
				new(dbfakes.FakeBuild),
			}
			returnedBuilds := []db.Build{
				inFlightBuilds[0],
				inFlightBuilds[1],
				inFlightBuilds[2],
			}

			fakeBuildFactory.GetAllStartedBuildsReturns(returnedBuilds, nil)

			engineBuilds = []*enginefakes.FakeRunnable{}
			fakeEngine.NewBuildStub = func(build db.Build) engine.Runnable {
				engineBuild := new(enginefakes.FakeRunnable)
				engineBuilds = append(engineBuilds, engineBuild)
				return engineBuild
			}
		})

		It("resumes all currently in-flight builds", func() {
			tracker.Track()

			Eventually(engineBuilds[0].RunCallCount).Should(Equal(1))
			Eventually(engineBuilds[1].RunCallCount).Should(Equal(1))
			Eventually(engineBuilds[2].RunCallCount).Should(Equal(1))
		})
	})

	Describe("Release", func() {
		It("releases all builds tracked by engine", func() {
			tracker.Release()

			Expect(fakeEngine.ReleaseAllCallCount()).To(Equal(1))
		})
	})
})
