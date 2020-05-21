package component_test

import (
	"context"
	"errors"
	"testing"

	"github.com/concourse/concourse/atc/component"
	"github.com/concourse/concourse/atc/component/cmocks"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestCoordinator(t *testing.T) {
	suite.Run(t, &CoordinatorSuite{
		Assertions: require.New(t),
	})
}

type CoordinatorSuite struct {
	suite.Suite
	*require.Assertions
}

type CoordinatorTest struct {
	It string

	LockAvailable bool
	LockErr       error

	Disappeared bool
	ReloadErr   error

	Paused          bool
	IntervalElapsed bool

	Runs   bool
	RunErr error

	UpdatesLastRan   bool
	UpdateLastRanErr error
}

func (test CoordinatorTest) Run(s *CoordinatorSuite, action func(*component.Coordinator, context.Context)) {
	fakeLocker := new(lockfakes.FakeLockFactory)
	fakeComponent := new(cmocks.Component)
	fakeRunnable := new(cmocks.Runnable)

	var fakeLock *lockfakes.FakeLock
	if test.LockAvailable {
		fakeLock = new(lockfakes.FakeLock)
		fakeLocker.AcquireReturns(fakeLock, true, nil)
	} else {
		fakeLocker.AcquireReturns(nil, false, test.LockErr)
	}

	componentName := "some-name"

	fakeComponent.On("Name").Return(componentName)
	fakeComponent.On("Paused").Return(test.Paused)
	fakeComponent.On("IntervalElapsed").Return(test.IntervalElapsed)
	fakeComponent.On("UpdateLastRan").Return(test.UpdateLastRanErr)

	fakeComponent.On("Reload").Return(!test.Disappeared, test.ReloadErr).Run(func(mock.Arguments) {
		// make sure we haven't asked for anything prior to reloading
		fakeComponent.AssertNotCalled(s.T(), "Paused")
		fakeComponent.AssertNotCalled(s.T(), "IntervalElapsed")
	})

	ctx := context.Background()

	if test.Runs {
		fakeRunnable.On("Run", ctx).Return(test.RunErr).Run(func(mock.Arguments) {
			// make sure the lock is held while running
			s.Equal(fakeLock.ReleaseCallCount(), 0, "lock was released too early")

			// make sure we haven't updated this too early
			fakeComponent.AssertNotCalled(s.T(), "UpdateLastRan")
		})
	}

	coordinator := &component.Coordinator{
		Locker:    fakeLocker,
		Component: fakeComponent,
		Runnable:  fakeRunnable,
	}

	action(coordinator, ctx)

	if test.Runs {
		fakeRunnable.AssertCalled(s.T(), "Run", ctx)
	} else {
		fakeRunnable.AssertNotCalled(s.T(), "Run")
	}

	if test.UpdatesLastRan {
		fakeComponent.AssertCalled(s.T(), "UpdateLastRan")
	} else {
		fakeComponent.AssertNotCalled(s.T(), "UpdateLastRan")
	}

	// broadly assert that the lock is released as this should apply to any code
	// branch that allowed the lock to be acquired
	if test.LockAvailable {
		_, acquiredLock := fakeLocker.AcquireArgsForCall(0)
		s.Equal(lock.NewTaskLockID(componentName), acquiredLock, "acquired wrong lock")

		s.Equal(1, fakeLock.ReleaseCallCount(), "lock was not released")
	}
}

func (s *CoordinatorSuite) TestRunPeriodically() {
	someErr := errors.New("oh noes")

	for _, t := range []CoordinatorTest{
		{
			It: "runs if the lock is available and the interval elapsed",

			LockAvailable:   true,
			IntervalElapsed: true,

			Runs:           true,
			UpdatesLastRan: true,
		},
		{
			It: "does not run if lock is unavailable",

			LockAvailable:   false,
			IntervalElapsed: true,

			Runs: false,
		},
		{
			It: "does not run if acquiring the lock errors",

			LockErr:         someErr,
			IntervalElapsed: true,

			Runs: false,
		},
		{
			It: "does not run if the component disappears while reloading",

			LockAvailable:   true,
			IntervalElapsed: true,
			Disappeared:     true,

			Runs: false,
		},
		{
			It: "does not run if reloading the component errors",

			LockAvailable: true,
			ReloadErr:     someErr,

			Runs: false,
		},
		{
			It: "does not run if the lock is available but the interval has not elapsed",

			LockAvailable:   true,
			IntervalElapsed: false,

			Runs: false,
		},
		{
			It: "does not run if the component is paused",

			LockAvailable:   true,
			Paused:          true,
			IntervalElapsed: true,

			Runs: false,
		},
		{
			It: "does not update last ran if running failed",

			LockAvailable:   true,
			IntervalElapsed: true,

			Runs:           true,
			RunErr:         someErr,
			UpdatesLastRan: false,
		},
	} {
		s.Run(t.It, func() {
			t.Run(s, (*component.Coordinator).RunPeriodically)
		})
	}
}

func (s *CoordinatorSuite) TestRunImmediately() {
	someErr := errors.New("oh noes")

	for _, t := range []CoordinatorTest{
		{
			It: "runs if the lock is available and the interval elapsed",

			LockAvailable:   true,
			IntervalElapsed: true,

			Runs:           true,
			UpdatesLastRan: true,
		},
		{
			It: "runs if the lock is available even if the interval has not elapsed",

			LockAvailable:   true,
			IntervalElapsed: false,

			Runs:           true,
			UpdatesLastRan: true,
		},
		{
			It: "does not run if lock is unavailable",

			LockAvailable: false,

			Runs: false,
		},
		{
			It: "does not run if acquiring the lock errors",

			LockErr: someErr,

			Runs: false,
		},
		{
			It: "does not run if reloading the component errors",

			LockAvailable: true,
			ReloadErr:     someErr,

			Runs: false,
		},
		{
			It: "does not run if the component disappeared",

			LockAvailable:   true,
			Disappeared:     true,
			IntervalElapsed: true,

			Runs:           false,
			UpdatesLastRan: false,
		},
		{
			It: "does not run if the component is paused",

			LockAvailable:   true,
			Paused:          true,
			IntervalElapsed: true,

			Runs: false,
		},
	} {
		s.Run(t.It, func() {
			t.Run(s, (*component.Coordinator).RunImmediately)
		})
	}
}
