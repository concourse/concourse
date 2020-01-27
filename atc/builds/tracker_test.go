package builds_test

import (
	"time"

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
		var engineBuilds chan *enginefakes.FakeRunnable

		var trackErr error

		BeforeEach(func() {
			fakeBuild1 := new(dbfakes.FakeBuild)
			fakeBuild1.IDReturns(1)
			fakeBuild2 := new(dbfakes.FakeBuild)
			fakeBuild2.IDReturns(2)
			fakeBuild3 := new(dbfakes.FakeBuild)
			fakeBuild3.IDReturns(3)

			inFlightBuilds = []*dbfakes.FakeBuild{
				fakeBuild1,
				fakeBuild2,
				fakeBuild3,
			}

			returnedBuilds := []db.Build{
				inFlightBuilds[0],
				inFlightBuilds[1],
				inFlightBuilds[2],
			}

			fakeBuildFactory.GetAllStartedBuildsReturns(returnedBuilds, nil)

			engineBuilds = make(chan *enginefakes.FakeRunnable, 3)
			fakeEngine.NewBuildStub = func(build db.Build) engine.Runnable {
				engineBuild := new(enginefakes.FakeRunnable)
				engineBuilds <- engineBuild
				return engineBuild
			}
		})

		JustBeforeEach(func() {
			trackErr = tracker.Track()
		})

		It("succeeds", func() {
			Expect(trackErr).NotTo(HaveOccurred())
		})

		It("runs all currently in-flight builds", func() {
			Eventually((<-engineBuilds).RunCallCount).Should(Equal(1))
			Eventually((<-engineBuilds).RunCallCount).Should(Equal(1))
			Eventually((<-engineBuilds).RunCallCount).Should(Equal(1))
		})

		Context("when a build is already being tracked", func() {
			BeforeEach(func() {
				fakeBuild := new(dbfakes.FakeBuild)
				fakeBuild.IDReturns(1)

				fakeEngine.NewBuildStub = func(build db.Build) engine.Runnable {
					time.Sleep(time.Second)
					return new(enginefakes.FakeRunnable)
				}

				fakeBuildFactory.GetAllStartedBuildsReturns([]db.Build{
					fakeBuild,
					fakeBuild,
				}, nil)
			})

			It("succeeds", func() {
				Expect(trackErr).NotTo(HaveOccurred())
			})

			It("runs only one pending build", func() {
				Eventually(fakeEngine.NewBuildCallCount).Should(Equal(1))
			})
		})
	})

	Describe("Release", func() {
		It("releases all builds tracked by engine", func() {
			tracker.Release()

			Expect(fakeEngine.ReleaseAllCallCount()).To(Equal(1))
		})
	})
})
