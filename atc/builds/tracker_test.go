package builds_test

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/builds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/engine/enginefakes"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TrackerSuite struct {
	suite.Suite
	*require.Assertions

	fakeBuildFactory *dbfakes.FakeBuildFactory
	fakeEngine       *enginefakes.FakeEngine

	tracker *builds.Tracker
}

func (s *TrackerSuite) SetupTest() {
	s.fakeBuildFactory = new(dbfakes.FakeBuildFactory)
	s.fakeEngine = new(enginefakes.FakeEngine)

	s.tracker = builds.NewTracker(
		s.fakeBuildFactory,
		s.fakeEngine,
	)
}

func (s *TrackerSuite) TestTrackRunsStartedBuilds() {
	startedBuilds := []db.Build{}
	for i := 0; i < 3; i++ {
		fakeBuild := new(dbfakes.FakeBuild)
		fakeBuild.IDReturns(i + 1)
		startedBuilds = append(startedBuilds, fakeBuild)
	}

	s.fakeBuildFactory.GetAllStartedBuildsReturns(startedBuilds, nil)

	running := make(chan db.Build, 3)
	s.fakeEngine.NewBuildStub = func(build db.Build) engine.Runnable {
		engineBuild := new(enginefakes.FakeRunnable)
		engineBuild.RunStub = func(lager.Logger) {
			running <- build
		}

		return engineBuild
	}

	err := s.tracker.Run(context.TODO())
	s.NoError(err)

	ranBuilds := []db.Build{
		<-running,
		<-running,
		<-running,
	}
	s.ElementsMatch(startedBuilds, ranBuilds)
}

func (s *TrackerSuite) TestTrackDoesntTrackAlreadyRunningBuilds() {
	fakeBuild := new(dbfakes.FakeBuild)
	fakeBuild.IDReturns(1)
	s.fakeBuildFactory.GetAllStartedBuildsReturns([]db.Build{fakeBuild}, nil)

	wait := make(chan struct{})
	defer close(wait)

	running := make(chan db.Build, 3)
	s.fakeEngine.NewBuildStub = func(build db.Build) engine.Runnable {
		engineBuild := new(enginefakes.FakeRunnable)
		engineBuild.RunStub = func(lager.Logger) {
			running <- build
			<-wait
		}

		return engineBuild
	}

	err := s.tracker.Run(context.TODO())
	s.NoError(err)

	<-running

	err = s.tracker.Run(context.TODO())
	s.NoError(err)

	select {
	case <-running:
		s.Fail("another build was started!")
	case <-time.After(100 * time.Millisecond):
	}
}
