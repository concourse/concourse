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
	ReapVolume(handle string) error
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

func (vf *volumeFactory) BuildWithIndefiniteTTL(logger lager.Logger, bcVol baggageclaim.Volume) (Volume, error) { // TODO think about this method
	logger = logger.WithData(lager.Data{"volume": bcVol.Handle()})

	bcVol.Release(nil)

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
