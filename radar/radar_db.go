package radar

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . RadarDB

type RadarDB interface {
	GetPipelineName() string
	GetPipelineID() int
	ScopedName(string) string
	TeamID() int

	IsPaused() (bool, error)

	GetConfig() (atc.Config, db.ConfigVersion, bool, error)

	GetLatestVersionedResource(resourceName string) (db.SavedVersionedResource, bool, error)
	GetResource(resourceName string) (db.SavedResource, bool, error)
	GetResourceType(resourceTypeName string) (db.SavedResourceType, bool, error)
	PauseResource(resourceName string) error
	UnpauseResource(resourceName string) error

	SaveResourceVersions(atc.ResourceConfig, []atc.Version) error
	SaveResourceTypeVersion(atc.ResourceType, atc.Version) error
	SetResourceCheckError(resource db.SavedResource, err error) error
	LeaseResourceChecking(logger lager.Logger, resource string, interval time.Duration, immediate bool) (db.Lease, bool, error)
	LeaseResourceTypeChecking(logger lager.Logger, resourceType string, interval time.Duration, immediate bool) (db.Lease, bool, error)
}
