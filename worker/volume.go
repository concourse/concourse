package worker

import (
	"sync"
	"time"

	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

const volumeKeepalive = 30 * time.Second

//go:generate counterfeiter . VolumeFactoryDB
type VolumeFactoryDB interface {
	GetVolumeTTL(volumeHandle string) (time.Duration, error)
	ReapVolume(handle string) error
}

//go:generate counterfeiter . VolumeFactory
type VolumeFactory interface {
	Build(baggageclaim.Volume) (Volume, error)
}

type volumeFactory struct {
	db     VolumeFactoryDB
	clock  clock.Clock
	logger lager.Logger
}

func NewVolumeFactory(logger lager.Logger, db VolumeFactoryDB, clock clock.Clock) VolumeFactory {
	return &volumeFactory{
		logger: logger,
		db:     db,
		clock:  clock,
	}
}

func (vf *volumeFactory) Build(bcVol baggageclaim.Volume) (Volume, error) {
	bcVol.Release(0)

	vol := newVolume(vf.logger, bcVol, vf.clock, vf.db)

	return vol, nil
}

//go:generate counterfeiter . Volume

type Volume interface {
	baggageclaim.Volume
}

type volume struct {
	baggageclaim.Volume
	db VolumeFactoryDB

	release      chan time.Duration
	heartbeating *sync.WaitGroup
	releaseOnce  sync.Once
}

func newVolume(logger lager.Logger, bcVol baggageclaim.Volume, clock clock.Clock, db VolumeFactoryDB) Volume {
	vol := &volume{
		Volume: bcVol,
		db:     db,

		heartbeating: new(sync.WaitGroup),
		release:      make(chan time.Duration, 1),
	}

	ttl, err := vol.db.GetVolumeTTL(vol.Handle())
	if err != nil {
		logger.Info("failed-to-lookup-ttl", lager.Data{"error": err.Error()})
		ttl, _, err = bcVol.Expiration()

		if err != nil {
			logger.Error("failed-to-lookup-expiration-of-volume", err)
			return nil
		}
	}

	vol.heartbeat(logger.Session("initial-heartbeat"), ttl)

	vol.heartbeating.Add(1)
	go vol.heartbeatContinuously(
		logger.Session("continuos-heartbeat"),
		clock.NewTicker(volumeKeepalive),
		ttl,
	)

	return vol
}

func (v *volume) Release(finalTTL time.Duration) {
	v.releaseOnce.Do(func() {
		v.release <- finalTTL
		v.heartbeating.Wait()
	})

	return
}

func (v *volume) heartbeatContinuously(logger lager.Logger, pacemaker clock.Ticker, initialTTL time.Duration) {
	defer v.heartbeating.Done()
	defer pacemaker.Stop()

	logger.Debug("start")
	defer logger.Debug("done")

	ttlToSet := initialTTL
	for {
		select {
		case <-pacemaker.C():
			ttl, err := v.db.GetVolumeTTL(v.Handle())
			if err != nil {
				logger.Info("failed-to-lookup-ttl", lager.Data{"error": err.Error()})
			} else {
				ttlToSet = ttl
			}
			v.heartbeat(logger.Session("tick"), ttlToSet)

		case finalTTL := <-v.release:
			if finalTTL != 0 {
				v.heartbeat(logger.Session("final"), finalTTL)
			}

			return
		}
	}
}

func (v *volume) heartbeat(logger lager.Logger, ttl time.Duration) {
	logger.Debug("start")
	defer logger.Debug("done")

	err := v.SetTTL(ttl)
	if err != nil {
		if err == baggageclaim.ErrVolumeNotFound {
			v.db.ReapVolume(v.Handle())
		}
		logger.Error("failed-to-heartbeat-to-volume", err)
	}
}
