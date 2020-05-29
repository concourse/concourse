package component_test

import (
	"context"
	"os"
	"testing"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/component"
	"github.com/concourse/concourse/atc/component/cmocks"
	"github.com/concourse/concourse/atc/db"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tedsuo/ifrit"
)

func TestRunner(t *testing.T) {
	suite.Run(t, &RunnerSuite{
		Assertions: require.New(t),
	})
}

type RunnerSuite struct {
	suite.Suite
	*require.Assertions

	clock *fakeclock.FakeClock
}

func (s *RunnerSuite) SetupTest() {
	s.clock = fakeclock.NewFakeClock(time.Now())
	component.Clock = s.clock
}

func (s *RunnerSuite) TearDownTest() {
	component.Clock = clock.NewClock()
}

func (s *RunnerSuite) TestEndToEnd() {
	interval := 30 * time.Second
	componentName := "some-component"

	mockComponent := new(cmocks.Component)
	mockComponent.On("Name").Return(componentName)
	mockComponent.On("Interval").Return(interval)

	mockBus := new(cmocks.NotificationsBus)

	ranPeriodically := make(chan context.Context)
	ranImmediately := make(chan context.Context)

	mockSchedulable := schedulable{
		runPeriodically: func(ctx context.Context) {
			ranPeriodically <- ctx
		},
		runImmediately: func(ctx context.Context) {
			ranImmediately <- ctx
		},
	}

	scheduler := &component.Runner{
		Logger:      lagertest.NewTestLogger("test"),
		Interval:    interval,
		Component:   mockComponent,
		Bus:         mockBus,
		Schedulable: mockSchedulable,
	}

	notifications := make(chan db.Notification, 1)

	var process ifrit.Process
	s.Run("listens for component notifications on start", func() {
		mockBus.On("Listen", componentName, db.DontQueueNotifications).Return(notifications, nil)

		process = ifrit.Background(scheduler)
		select {
		case <-process.Ready():
		case err := <-process.Wait():
			s.Failf("process exited early", "error: %s", err)
		}

		mockBus.AssertCalled(s.T(), "Listen", componentName, db.DontQueueNotifications)
	})

	defer func() {
		process.Signal(os.Interrupt)
		<-process.Wait()
	}()

	s.Run("runs periodically on component interval", func() {
		s.clock.WaitForWatcherAndIncrement(interval)
		<-ranPeriodically
		s.Empty(ranImmediately)

		s.clock.WaitForWatcherAndIncrement(interval)
		<-ranPeriodically
		s.Empty(ranImmediately)
	})

	s.Run("runs immediately on notification bus events", func() {
		notifications <- db.Notification{Healthy: true}
		s.Empty(ranPeriodically)
		<-ranImmediately

		notifications <- db.Notification{Healthy: true}
		s.Empty(ranPeriodically)
		<-ranImmediately
	})

	s.Run("notifications reset the timer to prevent doing extra work", func() {
		// increment timer to just under the interval
		s.clock.WaitForWatcherAndIncrement(interval - 1)

		// send a notification instead
		notifications <- db.Notification{Healthy: true}
		s.Empty(ranPeriodically)
		<-ranImmediately

		// pass the remaining time
		s.clock.WaitForWatcherAndIncrement(1)

		// send few notifications to ensure a chance to fire the periodic timer
		notifications <- db.Notification{Healthy: true}
		s.Empty(ranPeriodically)
		<-ranImmediately
		notifications <- db.Notification{Healthy: true}
		s.Empty(ranPeriodically)
		<-ranImmediately

		// increment the timer the full amount
		s.clock.WaitForWatcherAndIncrement(interval)
		<-ranPeriodically
		s.Empty(ranImmediately)
	})

	s.Run("unlistens on exit", func() {
		mockBus.On("Unlisten", componentName, notifications).Return(nil)
		process.Signal(os.Interrupt)

		s.NoError(<-process.Wait())
		mockBus.AssertCalled(s.T(), "Unlisten", componentName, notifications)
	})
}

type schedulable struct {
	runPeriodically func(context.Context)
	runImmediately  func(context.Context)
}

func (s schedulable) RunPeriodically(ctx context.Context) {
	s.runPeriodically(ctx)
}

func (s schedulable) RunImmediately(ctx context.Context) {
	s.runImmediately(ctx)
}
