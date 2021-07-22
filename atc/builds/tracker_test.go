package builds_test

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	"github.com/concourse/concourse/atc/builds"
	"github.com/concourse/concourse/atc/builds/buildsfakes"
	"github.com/concourse/concourse/atc/component"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/util"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func init() {
	util.PanicSink = ioutil.Discard
}

type TrackerSuite struct {
	suite.Suite
	*require.Assertions

	fakeBuildFactory *dbfakes.FakeBuildFactory
	fakeEngine       *buildsfakes.FakeEngine

	tracker *builds.Tracker
}

func TestTracker(t *testing.T) {
	suite.Run(t, &TrackerSuite{
		Assertions: require.New(t),
	})
}

func (s *TrackerSuite) SetupTest() {
	s.fakeBuildFactory = new(dbfakes.FakeBuildFactory)
	s.fakeEngine = new(buildsfakes.FakeEngine)

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
	s.fakeEngine.NewBuildStub = func(build db.Build) builds.Runnable {
		engineBuild := new(buildsfakes.FakeRunnable)
		engineBuild.RunStub = func(context.Context) {
			running <- build
		}

		return engineBuild
	}

	err := s.tracker.Run(context.TODO())
	s.NoError(err)

	s.ElementsMatch([]int{
		startedBuilds[0].ID(),
		startedBuilds[1].ID(),
		startedBuilds[2].ID(),
	}, []int{
		(<-running).ID(),
		(<-running).ID(),
		(<-running).ID(),
	})
}

func (s *TrackerSuite) TestTrackerDoesntCrashWhenOneBuildPanic() {
	startedBuilds := []db.Build{}
	fakeBuild1 := new(dbfakes.FakeBuild)
	fakeBuild1.IDReturns(1)
	startedBuilds = append(startedBuilds, fakeBuild1)

	// build 2 and 3 are normal running build
	for i := 1; i < 3; i++ {
		fakeBuild := new(dbfakes.FakeBuild)
		fakeBuild.IDReturns(i + 1)
		startedBuilds = append(startedBuilds, fakeBuild)
	}

	s.fakeBuildFactory.GetAllStartedBuildsReturns(startedBuilds, nil)

	running := make(chan db.Build, 3)
	s.fakeEngine.NewBuildStub = func(build db.Build) builds.Runnable {
		fakeEngineBuild := new(buildsfakes.FakeRunnable)
		fakeEngineBuild.RunStub = func(context.Context) {
			if build.ID() == 1 {
				panic("something went wrong")
			} else {
				running <- build
			}
		}

		return fakeEngineBuild
	}

	err := s.tracker.Run(context.TODO())
	s.NoError(err)

	s.ElementsMatch([]int{
		startedBuilds[1].ID(),
		startedBuilds[2].ID(),
	}, []int{
		(<-running).ID(),
		(<-running).ID(),
	})

	s.Eventually(func() bool {
		return fakeBuild1.FinishCallCount() == 1
	}, time.Second, 10*time.Millisecond)

	s.Eventually(func() bool {
		return fakeBuild1.FinishArgsForCall(0) == db.BuildStatusErrored
	}, time.Second, 10*time.Millisecond)
}

func (s *TrackerSuite) TestTrackDoesntTrackAlreadyRunningBuilds() {
	fakeBuild := new(dbfakes.FakeBuild)
	fakeBuild.IDReturns(1)
	s.fakeBuildFactory.GetAllStartedBuildsReturns([]db.Build{fakeBuild}, nil)

	wait := make(chan struct{})
	defer close(wait)

	running := make(chan db.Build, 3)
	s.fakeEngine.NewBuildStub = func(build db.Build) builds.Runnable {
		engineBuild := new(buildsfakes.FakeRunnable)
		engineBuild.RunStub = func(context.Context) {
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

func (s *TrackerSuite) TestTrackerDrainsEngine() {
	var _ component.Drainable = s.tracker

	ctx := context.TODO()
	s.tracker.Drain(ctx)
	s.Equal(1, s.fakeEngine.DrainCallCount())
	s.Equal(ctx, s.fakeEngine.DrainArgsForCall(0))
}
