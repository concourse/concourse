package builds_test

import (
	"code.cloudfoundry.org/lager"
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
		var engineBuilds []*enginefakes.FakeBuild

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

			engineBuilds = []*enginefakes.FakeBuild{}
			fakeEngine.LookupBuildStub = func(logger lager.Logger, build db.Build) engine.Build {
				engineBuild := new(enginefakes.FakeBuild)
				engineBuilds = append(engineBuilds, engineBuild)
				return engineBuild
			}
		})

		It("resumes all currently in-flight builds", func() {
			tracker.Track()

			Eventually(engineBuilds[0].ResumeCallCount).Should(Equal(1))
			Eventually(engineBuilds[1].ResumeCallCount).Should(Equal(1))
			Eventually(engineBuilds[2].ResumeCallCount).Should(Equal(1))
		})
	})

	Describe("Release", func() {
		It("releases all builds tracked by engine", func() {
			tracker.Release()

			Expect(fakeEngine.ReleaseAllCallCount()).To(Equal(1))
		})
	})
})
