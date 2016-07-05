package worker

import (
	"github.com/concourse/atc/db"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . VolumeClient

type VolumeClient interface {
	FindVolume(lager.Logger, VolumeSpec) (Volume, bool, error)
	CreateVolume(lager.Logger, VolumeSpec) (Volume, error)
	ListVolumes(lager.Logger, VolumeProperties) ([]Volume, error)
	LookupVolume(lager.Logger, string) (Volume, bool, error)
}

type volumeClient struct {
	baggageclaimClient baggageclaim.Client
	db                 GardenWorkerDB
	volumeFactory      VolumeFactory
	workerName         string
}

func NewVolumeClient(
	baggageclaimClient baggageclaim.Client,
	db GardenWorkerDB,
	volumeFactory VolumeFactory,
	workerName string,
) VolumeClient {
	return &volumeClient{
		baggageclaimClient: baggageclaimClient,
		db:                 db,
		volumeFactory:      volumeFactory,
		workerName:         workerName,
	}
}

func (c *volumeClient) FindVolume(
	logger lager.Logger,
	volumeSpec VolumeSpec,
) (Volume, bool, error) {
	if c.baggageclaimClient == nil {
		return nil, false, ErrNoVolumeManager
	}

	volumeIdentifier := volumeSpec.Strategy.dbIdentifier()
	savedVolumes, err := c.db.GetVolumesByIdentifier(volumeIdentifier)
	if err != nil {
		return nil, false, err
	}

	if len(savedVolumes) == 0 {
		err = ErrMissingVolume
		logger.Error("failed-to-find-volume-in-db", err)
		return nil, false, err
	}

	if len(savedVolumes) > 1 {
		for i := 1; i < len(savedVolumes); i++ {
			handle := savedVolumes[i].Volume.Handle
			c.expireVolume(logger, handle)
		}
	}

	savedVolume := savedVolumes[0]

	volume, found, err := c.LookupVolume(logger, savedVolume.Handle)
	if !found {
		c.db.ReapVolume(savedVolume.Handle)
	}

	return volume, found, err
}

func (c *volumeClient) CreateVolume(
	logger lager.Logger,
	volumeSpec VolumeSpec,
) (Volume, error) {
	if c.baggageclaimClient == nil {
		return nil, ErrNoVolumeManager
	}

	bcVolume, err := c.baggageclaimClient.CreateVolume(
		logger.Session("create-volume"),
		volumeSpec.baggageclaimVolumeSpec(),
	)
	if err != nil {
		logger.Error("failed-to-create-volume", err)
		return nil, err
	}

	err = c.db.InsertVolume(db.Volume{
		Handle:     bcVolume.Handle(),
		WorkerName: c.workerName,
		TTL:        volumeSpec.TTL,
		Identifier: volumeSpec.Strategy.dbIdentifier(),
	})
	if err != nil {
		logger.Error("failed-to-save-volume-to-db", err)
		return nil, err
	}

	volume, found, err := c.volumeFactory.Build(logger, bcVolume)
	if err != nil {
		logger.Error("failed-build-volume", err)
		return nil, err
	}

	if !found {
		err = ErrMissingVolume
		logger.Error("volume-expired-immediately", err)
		return nil, err
	}

	return volume, nil
}

func (c *volumeClient) ListVolumes(logger lager.Logger, properties VolumeProperties) ([]Volume, error) {
	if c.baggageclaimClient == nil {
		return []Volume{}, nil
	}

	bcVolumes, err := c.baggageclaimClient.ListVolumes(
		logger,
		baggageclaim.VolumeProperties(properties),
	)
	if err != nil {
		logger.Error("failed-to-list-volumes", err)
		return nil, err
	}

	volumes := []Volume{}
	for _, bcVolume := range bcVolumes {
		volume, found, err := c.volumeFactory.Build(logger, bcVolume)
		if err != nil {
			return []Volume{}, err
		}

		if !found {
			continue
		}

		volumes = append(volumes, volume)
	}

	return volumes, nil
}

func (c *volumeClient) LookupVolume(logger lager.Logger, handle string) (Volume, bool, error) {
	if c.baggageclaimClient == nil {
		return nil, false, nil
	}

	bcVolume, found, err := c.baggageclaimClient.LookupVolume(logger, handle)
	if err != nil {
		logger.Error("failed-to-lookup-volume", err)
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return c.volumeFactory.Build(logger, bcVolume)
}

func (c *volumeClient) expireVolume(logger lager.Logger, handle string) error {
	wVol, found, err := c.LookupVolume(logger, handle)
	if err != nil {
		logger.Debug("failed-to-look-up-volume", lager.Data{
			"handle": handle,
		})
		return err
	}

	if !found {
		logger.Debug("volume-not-found", lager.Data{
			"handle": handle,
		})
		return nil
	}

	wVol.Release(FinalTTL(VolumeTTL))

	return nil
}
