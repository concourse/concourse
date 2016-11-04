package worker

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/metric"
	"github.com/concourse/baggageclaim"
)

const volumeKeepalive = 30 * time.Second

//go:generate counterfeiter . VolumeFactoryDB

type VolumeFactoryDB interface {
	GetVolumeTTL(volumeHandle string) (time.Duration, bool, error)
	ReapVolume(handle string) error
	SetVolumeTTL(handle string, ttl time.Duration) error
}

//go:generate counterfeiter . VolumeFactory

type VolumeFactory interface {
	BuildWithIndefiniteTTL(lager.Logger, baggageclaim.Volume) (Volume, error)
}

type volumeFactory struct {
	db VolumeFactoryDB
}

func NewVolumeFactory(db VolumeFactoryDB) VolumeFactory {
	return &volumeFactory{
		db: db,
	}
}

func (vf *volumeFactory) BuildWithIndefiniteTTL(logger lager.Logger, bcVol baggageclaim.Volume) (Volume, error) {
	logger = logger.WithData(lager.Data{"volume": bcVol.Handle()})

	bcVol.Release(nil)

	err := bcVol.SetTTL(0)
	if err != nil {
		logger.Error("failed-to-set-volume-ttl-in-baggageclaim", err)
		return nil, err
	}

	err = vf.db.SetVolumeTTL(bcVol.Handle(), 0)
	if err != nil {
		logger.Error("failed-to-set-volume-ttl-in-db", err)
		return nil, err
	}

	vol := &volume{
		Volume: bcVol,
		db:     vf.db,
	}

	metric.TrackedVolumes.Inc()
	return vol, nil
}

//go:generate counterfeiter . Volume

type Volume interface {
	baggageclaim.Volume

	Destroy()
}

type volume struct {
	baggageclaim.Volume

	db VolumeFactoryDB
}

type VolumeMount struct {
	Volume    Volume
	MountPath string
}

func (v *volume) Destroy() {
	v.Volume.Release(FinalTTL(0 * time.Second))
}
