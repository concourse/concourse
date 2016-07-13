package worker

import (
	"sync"
	"time"

	"github.com/concourse/atc/metric"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

const volumeKeepalive = 30 * time.Second

//go:generate counterfeiter . VolumeFactoryDB

type VolumeFactoryDB interface {
	GetVolumeTTL(volumeHandle string) (time.Duration, bool, error)
	ReapVolume(handle string) error
	SetVolumeTTL(string, time.Duration) error
	SetVolumeSizeInBytes(string, int64) error
}

//go:generate counterfeiter . VolumeFactory

type VolumeFactory interface {
	Build(lager.Logger, baggageclaim.Volume) (Volume, bool, error)
}

type volumeFactory struct {
	db    VolumeFactoryDB
	clock clock.Clock
}

func NewVolumeFactory(db VolumeFactoryDB, clock clock.Clock) VolumeFactory {
	return &volumeFactory{
		db:    db,
		clock: clock,
	}
}

func (vf *volumeFactory) Build(logger lager.Logger, bcVol baggageclaim.Volume) (Volume, bool, error) {
	bcVol.Release(nil)

	logger = logger.WithData(lager.Data{"volume": bcVol.Handle()})

	vol := &volume{
		Volume: bcVol,
		db:     vf.db,

		heartbeating: new(sync.WaitGroup),
		release:      make(chan *time.Duration, 1),
	}

	ttl, found, err := vf.db.GetVolumeTTL(vol.Handle())
	if err != nil {
		logger.Error("failed-to-lookup-expiration-of-volume", err)
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	vol.heartbeat(logger.Session("initial-heartbeat"), ttl)

	vol.heartbeating.Add(1)
	go vol.heartbeatContinuously(
		logger.Session("continuous-heartbeat"),
		vf.clock.NewTicker(volumeKeepalive),
		ttl,
	)

	metric.TrackedVolumes.Inc()

	return vol, true, nil
}

//go:generate counterfeiter . Volume

type Volume interface {
	baggageclaim.Volume

	// a noop method to ensure things aren't just returning baggageclaim.Volume
	HeartbeatingToDB()
}

type volume struct {
	baggageclaim.Volume

	db VolumeFactoryDB

	release      chan *time.Duration
	heartbeating *sync.WaitGroup
	releaseOnce  sync.Once
}

type VolumeMount struct {
	Volume    Volume
	MountPath string
}

func (*volume) HeartbeatingToDB() {}

func (v *volume) Release(finalTTL *time.Duration) {
	v.releaseOnce.Do(func() {
		v.release <- finalTTL
		v.heartbeating.Wait()
		metric.TrackedVolumes.Dec()
	})
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
			ttl, found, err := v.db.GetVolumeTTL(v.Handle())
			if err != nil {
				logger.Error("failed-to-lookup-volume-ttl", err)
			} else {
				if !found {
					logger.Info("volume-expired-from-database")
					return
				}

				ttlToSet = ttl
			}

			v.heartbeat(logger.Session("tick"), ttlToSet)

		case finalTTL := <-v.release:
			if finalTTL != nil {
				v.heartbeat(logger.Session("final"), *finalTTL)
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
			err = v.db.ReapVolume(v.Handle())
			if err != nil {
				logger.Error("failed-to-delete-volume-from-database", err)
			}
		}
		logger.Error("failed-to-heartbeat-to-volume", err)
	}

	err = v.db.SetVolumeTTL(v.Handle(), ttl)
	if err != nil {
		logger.Error("failed-to-heartbeat-to-database", err)
	}

	size, err := v.SizeInBytes()
	if err != nil {
		logger.Error("failed-to-get-volume-size", err)
	} else {
		err := v.db.SetVolumeSizeInBytes(v.Handle(), size)
		if err != nil {
			logger.Error("failed-to-store-volume-size", err)
		}
	}
}
