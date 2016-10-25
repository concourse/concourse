package worker

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/baggageclaim"
	uuid "github.com/nu7hatch/gouuid"
)

//go:generate counterfeiter . VolumeClient

type VolumeClient interface {
	FindVolume(lager.Logger, VolumeSpec) (Volume, bool, error)
	CreateVolume(logger lager.Logger, vs VolumeSpec, teamID int) (Volume, error)
	FindOrCreateVolumeForContainer(
		lager.Logger,
		VolumeSpec,
		*dbng.Worker,
		*dbng.CreatingContainer,
		*dbng.Team,
		string,
	) (Volume, error)
	FindOrCreateVolumeForBaseResourceType(
		lager.Logger,
		VolumeSpec,
		*dbng.Worker,
		*dbng.Team,
		string,
	) (Volume, error)
	ListVolumes(lager.Logger, VolumeProperties) ([]Volume, error)
	LookupVolume(lager.Logger, string) (Volume, bool, error)
}

var ErrVolumeExpiredImmediately = errors.New("volume expired immediately after saving")
var ErrCreatedVolumeNotFound = errors.New("failed-to-find-created-volume-in-baggageclaim")
var ErrBaseResourceTypeNotFound = errors.New("base-resource-type-not-found")

type volumeClient struct {
	baggageclaimClient        baggageclaim.Client
	db                        GardenWorkerDB
	volumeFactory             VolumeFactory
	dbVolumeFactory           dbng.VolumeFactory
	dbBaseResourceTypeFactory dbng.BaseResourceTypeFactory
	workerName                string
}

func NewVolumeClient(
	baggageclaimClient baggageclaim.Client,
	db GardenWorkerDB,
	volumeFactory VolumeFactory,
	dbVolumeFactory dbng.VolumeFactory,
	dbBaseResourceTypeFactory dbng.BaseResourceTypeFactory,
	workerName string,
) VolumeClient {
	return &volumeClient{
		baggageclaimClient:        baggageclaimClient,
		db:                        db,
		volumeFactory:             volumeFactory,
		dbVolumeFactory:           dbVolumeFactory,
		dbBaseResourceTypeFactory: dbBaseResourceTypeFactory,
		workerName:                workerName,
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

func (c *volumeClient) FindOrCreateVolumeForContainer(
	logger lager.Logger,
	volumeSpec VolumeSpec,
	worker *dbng.Worker,
	container *dbng.CreatingContainer,
	team *dbng.Team,
	mountPath string,
) (Volume, error) {
	return c.findOrCreateVolume(
		logger,
		volumeSpec,
		worker,
		team,
		func() (dbng.CreatingVolume, dbng.CreatedVolume, error) {
			return c.dbVolumeFactory.FindContainerVolume(team, worker, container, mountPath)
		},
		func() (dbng.CreatingVolume, error) {
			return c.dbVolumeFactory.CreateContainerVolume(team, worker, container, mountPath)
		},
	)
}

func (c *volumeClient) FindOrCreateVolumeForBaseResourceType(
	logger lager.Logger,
	volumeSpec VolumeSpec,
	worker *dbng.Worker,
	team *dbng.Team,
	resourceTypeName string,
) (Volume, error) {
	baseResourceType, found, err := c.dbBaseResourceTypeFactory.Find(resourceTypeName)
	if err != nil {
		return nil, err
	}
	if !found {
		logger.Error("base-resource-type-not-found", ErrBaseResourceTypeNotFound, lager.Data{"resource-type-name": resourceTypeName})
		return nil, ErrBaseResourceTypeNotFound
	}

	return c.findOrCreateVolume(
		logger,
		volumeSpec,
		worker,
		team,
		func() (dbng.CreatingVolume, dbng.CreatedVolume, error) {
			return c.dbVolumeFactory.FindBaseResourceTypeVolume(team, worker, baseResourceType)
		},
		func() (dbng.CreatingVolume, error) {
			return c.dbVolumeFactory.CreateBaseResourceTypeVolume(team, worker, baseResourceType)
		},
	)
}

func (c *volumeClient) CreateVolume(
	logger lager.Logger,
	volumeSpec VolumeSpec,
	teamID int,
) (Volume, error) {
	if c.baggageclaimClient == nil {
		return nil, ErrNoVolumeManager
	}

	handle, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	bcVolume, err := c.baggageclaimClient.CreateVolume(
		logger.Session("create-volume"),
		handle.String(),
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

	volume, err := c.volumeFactory.BuildWithIndefiniteTTL(logger, bcVolume)
	if err != nil {
		logger.Error("failed-build-volume", err)
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
		volume, err := c.volumeFactory.BuildWithIndefiniteTTL(logger, bcVolume)
		if err != nil {
			return []Volume{}, err
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

	volume, err := c.volumeFactory.BuildWithIndefiniteTTL(logger, bcVolume)
	if err != nil {
		return nil, false, err
	}

	return volume, true, nil
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

func (c *volumeClient) expireVolume(logger lager.Logger, handle string) error { // TODO consider removing this method?
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

	logger.Debug("releasing a volume " + handle + " [super logs]")

	wVol.Destroy()

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
