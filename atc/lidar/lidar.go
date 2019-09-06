package lidar

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
)

func NewRunner(
	logger lager.Logger,
	clock clock.Clock,
	scanRunner Runner,
	scanInterval time.Duration,
	checkRunner Runner,
	checkInterval time.Duration,
	notifications Notifications,
) ifrit.Runner {
	return grouper.NewParallel(
		os.Interrupt,
		[]grouper.Member{
			{
				Name:   "scanner",
				Runner: NewIntervalRunner(logger, clock, scanRunner, scanInterval, notifications, "scanner"),
			},
			{
				Name:   "checker",
				Runner: NewIntervalRunner(logger, clock, checkRunner, checkInterval, notifications, "checker"),
			},
		},
	)
}

func NewCheckerRunner(
	logger lager.Logger,
	clock clock.Clock,
	checkRunner Runner,
	checkInterval time.Duration,
	notifications Notifications,
) ifrit.Runner {
	return grouper.NewParallel(
		os.Interrupt,
		[]grouper.Member{
			{
				Name:   "checker",
				Runner: NewIntervalRunner(logger, clock, checkRunner, checkInterval, notifications, "checker"),
			},
		},
	)
}
