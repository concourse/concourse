package builds_test

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/enginefakes"
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
			fakeEngine.LookupBuildStub = func(logger lager.Logger, build db.Build) (engine.Build, error) {
				engineBuild := new(enginefakes.FakeBuild)
				engineBuilds = append(engineBuilds, engineBuild)
				return engineBuild, nil
			}
		})

		It("resumes all currently in-flight builds", func() {
			tracker.Track()

			Eventually(engineBuilds[0].ResumeCallCount).Should(Equal(1))
			Eventually(engineBuilds[1].ResumeCallCount).Should(Equal(1))
			Eventually(engineBuilds[2].ResumeCallCount).Should(Equal(1))
		})

		Context("when a build cannot be looked up", func() {
			BeforeEach(func() {
				fakeEngine.LookupBuildReturns(nil, errors.New("nope"))
			})

			It("saves its status as errored", func() {
				tracker.Track()

				Expect(inFlightBuilds[0].FinishWithErrorCallCount()).To(Equal(1))
				savedErr1 := inFlightBuilds[0].FinishWithErrorArgsForCall(0)
				Expect(savedErr1).To(Equal(errors.New("nope")))

				Expect(inFlightBuilds[1].FinishWithErrorCallCount()).To(Equal(1))
				savedErr2 := inFlightBuilds[1].FinishWithErrorArgsForCall(0)
				Expect(savedErr2).To(Equal(errors.New("nope")))

				Expect(inFlightBuilds[2].FinishWithErrorCallCount()).To(Equal(1))
				savedErr3 := inFlightBuilds[2].FinishWithErrorArgsForCall(0)
				Expect(savedErr3).To(Equal(errors.New("nope")))
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
