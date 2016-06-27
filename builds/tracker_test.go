package builds_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/builds/buildsfakes"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	enginefakes "github.com/concourse/atc/engine/enginefakes"
)

var _ = Describe("Tracker", func() {
	var (
		fakeTrackerDB *buildsfakes.FakeTrackerDB
		fakeEngine    *enginefakes.FakeEngine

		tracker *builds.Tracker
		logger  *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakeTrackerDB = new(buildsfakes.FakeTrackerDB)
		fakeEngine = new(enginefakes.FakeEngine)
		logger = lagertest.NewTestLogger("test")

		tracker = builds.NewTracker(
			logger,
			fakeTrackerDB,
			fakeEngine,
		)
	})

	Describe("Track", func() {
		var (
			inFlightBuilds []db.Build

			engineBuilds []*enginefakes.FakeBuild
		)

		BeforeEach(func() {
			inFlightBuilds = []db.Build{
				{ID: 1, PipelineID: 57},
				{ID: 2, PipelineID: 57},
				{ID: 3, PipelineID: 57},
			}

			engineBuilds = []*enginefakes.FakeBuild{
				new(enginefakes.FakeBuild),
				new(enginefakes.FakeBuild),
				new(enginefakes.FakeBuild),
			}

			fakeTrackerDB.GetAllStartedBuildsReturns(inFlightBuilds, nil)

			fakeEngine.LookupBuildStub = func(logger lager.Logger, build db.Build) (engine.Build, error) {
				return engineBuilds[build.ID-1], nil
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

				Expect(fakeTrackerDB.ErrorBuildCallCount()).To(Equal(3))

				savedBuilID1, savedPipelineID1, savedErr1 := fakeTrackerDB.ErrorBuildArgsForCall(0)
				Expect(savedBuilID1).To(Equal(1))
				Expect(savedPipelineID1).To(Equal(57))
				Expect(savedErr1).To(Equal(errors.New("nope")))

				savedBuilID2, savedPipelineID2, savedErr2 := fakeTrackerDB.ErrorBuildArgsForCall(1)
				Expect(savedBuilID2).To(Equal(2))
				Expect(savedPipelineID2).To(Equal(57))
				Expect(savedErr2).To(Equal(errors.New("nope")))

				savedBuilID3, savedPipelineID3, savedErr3 := fakeTrackerDB.ErrorBuildArgsForCall(2)
				Expect(savedBuilID3).To(Equal(3))
				Expect(savedPipelineID3).To(Equal(57))
				Expect(savedErr3).To(Equal(errors.New("nope")))
			})
		})
	})

})
