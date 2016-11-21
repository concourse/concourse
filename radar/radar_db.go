package radar

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
)

//go:generate counterfeiter . RadarDB

type RadarDB interface {
	GetPipelineName() string
	GetPipelineID() int
	ScopedName(string) string
	TeamID() int
	Config() atc.Config

	IsPaused() (bool, error)

	Reload() (bool, error)

	GetLatestVersionedResource(resourceName string) (db.SavedVersionedResource, bool, error)
	GetResource(resourceName string) (db.SavedResource, bool, error)
	GetResourceType(resourceTypeName string) (db.SavedResourceType, bool, error)
	PauseResource(resourceName string) error
	UnpauseResource(resourceName string) error

	SaveResourceVersions(atc.ResourceConfig, []atc.Version) error
	SaveResourceTypeVersion(atc.ResourceType, atc.Version) error
	SetResourceCheckError(resource db.SavedResource, err error) error
	AcquireResourceCheckingLock(logger lager.Logger, resource db.SavedResource, interval time.Duration, immediate bool) (lock.Lock, bool, error)
	AcquireResourceTypeCheckingLock(logger lager.Logger, resourceType db.SavedResourceType, interval time.Duration, immediate bool) (lock.Lock, bool, error)
}
