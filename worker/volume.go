package worker

import (
	"io"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/db"
	"github.com/concourse/baggageclaim"
)

//go:generate counterfeiter . Volume

type Volume interface {
	Handle() string
	Path() string

	SetProperty(key string, value string) error
	Properties() (baggageclaim.VolumeProperties, error)

	SetPrivileged(bool) error

	StreamIn(path string, tarStream io.Reader) error
	StreamOut(path string) (io.ReadCloser, error)

	COWStrategy() baggageclaim.COWStrategy

	InitializeResourceCache(*db.UsedResourceCache) error
	InitializeTaskCache(lager.Logger, int, string, string, bool) error

	CreateChildForContainer(db.CreatingContainer, string) (db.CreatingVolume, error)

	Destroy() error
}

type VolumeMount struct {
	Volume    Volume
	MountPath string
}

type volume struct {
	bcVolume     baggageclaim.Volume
	dbVolume     db.CreatedVolume
	volumeClient VolumeClient
}

func NewVolume(
	bcVolume baggageclaim.Volume,
	dbVolume db.CreatedVolume,
	volumeClient VolumeClient,
) Volume {
	return &volume{
		bcVolume:     bcVolume,
		dbVolume:     dbVolume,
		volumeClient: volumeClient,
	}
}

func (v *volume) Handle() string { return v.bcVolume.Handle() }

func (v *volume) Path() string { return v.bcVolume.Path() }

func (v *volume) SetProperty(key string, value string) error {
	return v.bcVolume.SetProperty(key, value)
}

func (v *volume) SetPrivileged(privileged bool) error {
	return v.bcVolume.SetPrivileged(privileged)
}

func (v *volume) StreamIn(path string, tarStream io.Reader) error {
	return v.bcVolume.StreamIn(path, tarStream)
}

func (v *volume) StreamOut(path string) (io.ReadCloser, error) {
	return v.bcVolume.StreamOut(path)
}

func (v *volume) Properties() (baggageclaim.VolumeProperties, error) {
	return v.bcVolume.Properties()
}

func (v *volume) Destroy() error {
	return v.bcVolume.Destroy()
}

func (v *volume) COWStrategy() baggageclaim.COWStrategy {
	return baggageclaim.COWStrategy{
		Parent: v.bcVolume,
	}
}

func (v *volume) InitializeResourceCache(urc *db.UsedResourceCache) error {
	return v.dbVolume.InitializeResourceCache(urc)
}

func (v *volume) InitializeTaskCache(
	logger lager.Logger,
	jobID int,
	stepName string,
	path string,
	privileged bool,
) error {
	if v.dbVolume.ParentHandle() == "" {
		return v.dbVolume.InitializeTaskCache(jobID, stepName, path)
	}

	logger.Debug("creating-an-import-volume", lager.Data{"path": v.bcVolume.Path()})

	// always create, if there are any existing task cache volumes they will be gced
	// after initialization of the current one
	importVolume, err := v.volumeClient.CreateVolumeForTaskCache(
		logger,
		VolumeSpec{
			Strategy:   baggageclaim.ImportStrategy{Path: v.bcVolume.Path()},
			Privileged: privileged,
		},
		v.dbVolume.TeamID(),
		jobID,
		stepName,
		path,
	)
	if err != nil {
		return err
	}

	return importVolume.InitializeTaskCache(logger, jobID, stepName, path, privileged)
}

func (v *volume) CreateChildForContainer(creatingContainer db.CreatingContainer, mountPath string) (db.CreatingVolume, error) {
	return v.dbVolume.CreateChildForContainer(creatingContainer, mountPath)
}
