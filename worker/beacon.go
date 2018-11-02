package worker

import (
	"context"
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/tsa"
)

type Beacon struct {
	Logger lager.Logger

	Client TSAClient

	RebalanceInterval time.Duration
	DrainTimeout      time.Duration

	LocalGardenNetwork string
	LocalGardenAddr    string

	LocalBaggageclaimNetwork string
	LocalBaggageclaimAddr    string
}

// total number of active registrations; all but one are "live", the rest
// should all be draining
const maxActiveRegistrations = 5

func (beacon *Beacon) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	beacon.Logger.Debug("start")
	defer beacon.Logger.Debug("done")

	var rebalanceCh <-chan time.Time
	if beacon.RebalanceInterval != 0 {
		ticker := time.NewTicker(beacon.RebalanceInterval)
		defer ticker.Stop()

		rebalanceCh = ticker.C
	}

	cwg := &countingWaitGroup{}
	defer cwg.Wait()

	rootCtx, cancelAll := context.WithCancel(lagerctx.NewContext(context.Background(), beacon.Logger))
	defer cancelAll()

	latestErrChan := make(chan error, 1)
	ctx, cancel := context.WithCancel(rootCtx)

	cwg.Add(1)
	go beacon.registerWorker(ctx, cwg, func() { close(ready) }, latestErrChan)

	for {
		select {
		case <-rebalanceCh:
			logger := beacon.Logger.Session("rebalance")

			if cwg.Count() >= maxActiveRegistrations {
				logger.Info("max-active-registrations-reached", lager.Data{
					"limit": maxActiveRegistrations,
				})

				continue
			} else {
				logger.Debug("rebalancing")
			}

			cancelPrev := cancel
			ctx, cancel = context.WithCancel(lagerctx.NewContext(rootCtx, logger))

			// make a new channel so prior registrations can write to their own
			// buffered channel and exit
			latestErrChan = make(chan error, 1)

			cwg.Add(1)
			go beacon.registerWorker(ctx, cwg, cancelPrev, latestErrChan)

		case err := <-latestErrChan:
			if err != nil {
				beacon.Logger.Error("exited-with-error", err)
			} else {
				beacon.Logger.Info("exited")
			}

			// not actually necessary since we defer cancel the root ctx, but makes
			// the linter happy
			cancel()

			return err

		case <-signals:
			beacon.Logger.Info("signalled")

			// not actually necessary since we defer cancel the root ctx, but makes
			// the linter happy
			cancel()

			return nil
		}
	}
}

func (beacon *Beacon) registerWorker(
	ctx context.Context,
	cwg *countingWaitGroup,
	registeredCb func(),
	errs chan<- error,
) {
	defer cwg.Done()

	logger := lagerctx.FromContext(ctx)

	errs <- beacon.Client.Register(ctx, tsa.RegisterOptions{
		LocalGardenNetwork: beacon.LocalGardenNetwork,
		LocalGardenAddr:    beacon.LocalGardenAddr,

		LocalBaggageclaimNetwork: beacon.LocalBaggageclaimNetwork,
		LocalBaggageclaimAddr:    beacon.LocalBaggageclaimAddr,

		DrainTimeout: beacon.DrainTimeout,

		RegisteredFunc: func() {
			logger.Info("registered")
			registeredCb()
		},

		HeartbeatedFunc: func() {
			logger.Info("heartbeated")
		},
	})
}
