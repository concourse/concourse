package lidar

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
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
	componentFactory db.ComponentFactory,
) ifrit.Runner {
	return grouper.NewParallel(
		os.Interrupt,
		[]grouper.Member{
			{
				Name:   atc.ComponentLidarScanner,
				Runner: NewIntervalRunner(logger, clock, scanRunner, scanInterval, notifications, atc.ComponentLidarScanner, componentFactory),
			},
			{
				Name:   atc.ComponentLidarChecker,
				Runner: NewIntervalRunner(logger, clock, checkRunner, checkInterval, notifications, atc.ComponentLidarChecker, componentFactory),
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
	componentFactory db.ComponentFactory,
) ifrit.Runner {
	return grouper.NewParallel(
		os.Interrupt,
		[]grouper.Member{
			{
				Name:   atc.ComponentLidarChecker,
				Runner: NewIntervalRunner(logger, clock, checkRunner, checkInterval, notifications, atc.ComponentLidarChecker, componentFactory),
			},
		},
	)
}
