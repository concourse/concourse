package worker

import (
	"errors"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/baggageclaim"
)

const creatingVolumeRetryDelay = 1 * time.Second

//go:generate counterfeiter . VolumeClient

type VolumeClient interface {
	FindVolume(lager.Logger, VolumeSpec) (Volume, bool, error)
	CreateVolume(logger lager.Logger, vs VolumeSpec, teamID int) (Volume, error)
	ListVolumes(lager.Logger, VolumeProperties) ([]Volume, error)
	LookupVolume(lager.Logger, string) (Volume, bool, error)
}

var ErrVolumeExpiredImmediately = errors.New("volume expired immediately after saving")

type volumeClient struct {
	baggageclaimClient baggageclaim.Client
	db                 GardenWorkerDB
	volumeFactory      VolumeFactory
	clock              clock.Clock
	workerName         string
}

func NewVolumeClient(
	baggageclaimClient baggageclaim.Client,
	db GardenWorkerDB,
	volumeFactory VolumeFactory,
	clock clock.Clock,
	workerName string,
) VolumeClient {
	return &volumeClient{
		baggageclaimClient:        baggageclaimClient,
		db:                        db,
		volumeFactory:             volumeFactory,
		dbVolumeFactory:           dbVolumeFactory,
		dbBaseResourceTypeFactory: dbBaseResourceTypeFactory,
		clock:      clock,
		workerName: workerName,
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
		return nil, false, nil
	}

	var savedVolume db.SavedVolume
	if len(savedVolumes) == 1 {
		savedVolume = savedVolumes[0]
	} else {
		savedVolume, err = c.selectLowestAlphabeticalVolume(logger, savedVolumes)
		if err != nil {
			return nil, false, err
		}
	}

	return c.LookupVolume(logger, savedVolume.Handle)
}

func (c *volumeClient) CreateVolume(
	logger lager.Logger,
	volumeSpec VolumeSpec,
	teamID int,
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
		TeamID:     teamID,
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
		err = ErrVolumeExpiredImmediately
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

func (c *volumeClient) selectLowestAlphabeticalVolume(logger lager.Logger, volumes []db.SavedVolume) (db.SavedVolume, error) {
	var lowestVolume db.SavedVolume

	for _, v := range volumes {
		if lowestVolume.ID == 0 {
			lowestVolume = v
		} else if v.ID < lowestVolume.ID {
			lowestVolume = v
		}
	}

	for _, v := range volumes {
		if v != lowestVolume {
			expLog := logger.Session("expiring-redundant-volume", lager.Data{
				"volume-handle": v.Handle,
			})

			err := c.expireVolume(expLog, v.Handle)
			if err != nil {
				return db.SavedVolume{}, err
			}
		}
	}

	return lowestVolume, nil
}

func (c *volumeClient) expireVolume(logger lager.Logger, handle string) error {
	logger.Info("expiring")

	wVol, found, err := c.LookupVolume(logger, handle)
	if err != nil {
		logger.Error("failed-to-look-up-volume", err)
		return err
	}

	if !found {
		logger.Debug("volume-already-gone")
		return nil
	}

	wVol.Release(FinalTTL(VolumeTTL))

	return nil
}

func (c *volumeClient) findOrCreateVolume(
	logger lager.Logger,
	volumeSpec VolumeSpec,
	worker *dbng.Worker,
	team *dbng.Team,
	findVolumeFunc func() (dbng.CreatingVolume, dbng.CreatedVolume, error),
	createVolumeFunc func() (dbng.CreatingVolume, error),
) (Volume, error) {
	var bcVolume baggageclaim.Volume
	var bcVolumeFound bool

	creatingVolume, createdVolume, err := findVolumeFunc()
	if err != nil {
		logger.Error("failed-to-find-volume-in-db", err)
		return nil, err
	}

	if createdVolume != nil {
		bcVolume, bcVolumeFound, err = c.baggageclaimClient.LookupVolume(
			logger.Session("lookup-volume"),
			createdVolume.Handle(),
		)
		if err != nil {
			logger.Error("failed-to-lookup-volume-in-baggageclaim", err)
			return nil, err
		}

		if !bcVolumeFound {
			logger.Error("failed-to-lookup-volume-in-baggageclaim", ErrCreatedVolumeNotFound)
			return nil, ErrCreatedVolumeNotFound
		}
	} else {
		if creatingVolume != nil {
			bcVolume, bcVolumeFound, err = c.baggageclaimClient.LookupVolume(
				logger.Session("create-volume"),
				creatingVolume.Handle(),
			)
			if err != nil {
				logger.Error("failed-to-lookup-volume-in-baggageclaim", err)
				return nil, err
			}
		} else {
			creatingVolume, err = createVolumeFunc()
			if err != nil {
				logger.Error("failed-to-create-volume-in-db", err)
				return nil, err
			}
		}

		lock, acquired, err := c.db.AcquireVolumeCreatingLock(logger, creatingVolume.ID())
		if err != nil {
			logger.Error("failed-to-acquire-volume-creating-lock", err)
			return nil, err
		}

		if !acquired {
			c.clock.Sleep(creatingVolumeRetryDelay)
			return c.findOrCreateVolume(logger, volumeSpec, worker, team, findVolumeFunc, createVolumeFunc)
		}

		defer lock.Release()

		if !bcVolumeFound {
			bcVolume, err = c.baggageclaimClient.CreateVolume(
				logger.Session("create-volume"),
				creatingVolume.Handle(),
				volumeSpec.baggageclaimVolumeSpec(),
			)
			if err != nil {
				logger.Error("failed-to-create-volume-in-baggageclaim", err)
				return nil, err
			}
		}

		_, err = creatingVolume.Created()
		if err != nil {
			logger.Error("failed-to-initialize-volume", err)
			return nil, err
		}
	}

	volume, err := c.volumeFactory.BuildWithIndefiniteTTL(logger, bcVolume)
	if err != nil {
		logger.Error("failed-build-volume", err)
		return nil, err
	}

	return volume, nil
}
