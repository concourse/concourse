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

	bwg := &waitGroupWithCount{}
	defer bwg.Wait()

	rootCtx, cancelAll := context.WithCancel(lagerctx.NewContext(context.Background(), beacon.Logger))
	defer cancelAll()

	registerWorker := func(ctx context.Context, registeredCb func(), errs chan<- error) {
		defer bwg.Decrement()

		logger := lagerctx.FromContext(ctx)

		errs <- beacon.Client.Register(ctx, tsa.RegisterOptions{
			LocalGardenNetwork: beacon.LocalGardenNetwork,
			LocalGardenAddr:    beacon.LocalGardenAddr,

			LocalBaggageclaimNetwork: beacon.LocalBaggageclaimNetwork,
			LocalBaggageclaimAddr:    beacon.LocalBaggageclaimAddr,

			DrainTimeout: beacon.RebalanceInterval,

			RegisteredFunc: func() {
				logger.Info("registered")
				registeredCb()
			},

			HeartbeatedFunc: func() {
				logger.Info("heartbeated")
			},
		})
	}

	latestErrChan := make(chan error, 1)
	cancellableCtx, cancelCurrent := context.WithCancel(rootCtx)

	bwg.Increment()
	go registerWorker(cancellableCtx, func() { close(ready) }, latestErrChan)

	for {
		select {
		case <-rebalanceCh:
			if bwg.Count() >= maxActiveRegistrations {
				beacon.Logger.Debug("max-active-registrations-reached", lager.Data{
					"limit": maxActiveRegistrations,
				})

				continue
			}

			rebalanceLogger := beacon.Logger.Session("rebalance")

			rebalanceLogger.Debug("rebalancing")

			bwg.Increment()

			rebalanceCtx := lagerctx.NewContext(rootCtx, rebalanceLogger)

			cancelPrev := cancelCurrent
			cancellableCtx, cancelCurrent = context.WithCancel(rebalanceCtx)

			latestErrChan = make(chan error, 1)
			go registerWorker(cancellableCtx, cancelPrev, latestErrChan)

		case err := <-latestErrChan:
			if err != nil {
				beacon.Logger.Error("exited-with-error", err)
			} else {
				beacon.Logger.Info("exited")
			}

			// not actually necessary since we defer cancel the root ctx, but makes
			// the linter happy
			cancelCurrent()

			return err

		case <-signals:
			beacon.Logger.Info("signalled")

			// not actually necessary since we defer cancel the root ctx, but makes
			// the linter happy
			cancelCurrent()

			return nil
		}
	}
}
