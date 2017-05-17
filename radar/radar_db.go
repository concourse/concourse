package radar

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

//go:generate counterfeiter . RadarDB

type RadarDB interface {
	ScopedName(string) string

	IsPaused() (bool, error)

	Reload() (bool, error)

	GetResource(resourceName string) (db.SavedResource, bool, error)
	GetResourceType(resourceTypeName string) (db.SavedResourceType, bool, error)

	SaveResourceVersions(atc.ResourceConfig, []atc.Version) error
	SaveResourceTypeVersion(atc.ResourceType, atc.Version) error
	SetResourceCheckError(resource db.SavedResource, err error) error
}
