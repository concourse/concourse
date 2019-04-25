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
	scanNotifier chan bool,
	checkRunner Runner,
	checkInterval time.Duration,
	checkNotifier chan bool,
) ifrit.Runner {
	return grouper.NewParallel(
		os.Interrupt,
		[]grouper.Member{
			{
				Name:   "scanner",
				Runner: NewIntervalRunner(logger, clock, scanRunner, scanInterval, scanNotifier),
			},
			{
				Name:   "checker",
				Runner: NewIntervalRunner(logger, clock, checkRunner, checkInterval, checkNotifier),
			},
		},
	)
}
