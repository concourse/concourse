package dbng

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db/lock"
)

type ResourceUser interface {
	UseResourceCache(lager.Logger, Tx, lock.LockFactory, ResourceCache) (*UsedResourceCache, error)
	UseResourceConfig(lager.Logger, Tx, lock.LockFactory, ResourceConfig) (*UsedResourceConfig, error)
}

type forBuild struct {
	BuildID int
}

func ForBuild(id int) ResourceUser {
	return forBuild{id}
}

func (user forBuild) UseResourceCache(logger lager.Logger, tx Tx, lockFactory lock.LockFactory, resourceCache ResourceCache) (*UsedResourceCache, error) {
	return resourceCache.findOrCreate(logger, tx, lockFactory, user, "build_id", user.BuildID)
}

// UseResourceConfig creates the ResourceConfig, recursively creating its
// parent ResourceConfig or BaseResourceType, and registers a "Use" for the
// given build.
//
// An `image_resource` or a `get` within a build will result in a
// UsedResourceConfig.
//
// ErrResourceConfigDisappeared may be returned if the resource config was
// found initially but was removed before we could use it.
//
// ErrResourceConfigAlreadyExists may be returned if a concurrent call resulted
// in a conflict.
//
// ErrResourceConfigParentDisappeared may be returned if the resource config's
// parent ResourceConfig or BaseResourceType was found initially but was
// removed before we could create the ResourceConfig.
//
// Each of these errors should result in the caller retrying from the start of
// the transaction.
func (user forBuild) UseResourceConfig(logger lager.Logger, tx Tx, lockFactory lock.LockFactory, resourceConfig ResourceConfig) (*UsedResourceConfig, error) {
	return resourceConfig.findOrCreate(logger, tx, lockFactory, user, "build_id", user.BuildID)
}

type forResource struct {
	ResourceID int
}

func ForResource(id int) ResourceUser {
	return forResource{id}
}

func (user forResource) UseResourceCache(logger lager.Logger, tx Tx, lockFactory lock.LockFactory, resourceCache ResourceCache) (*UsedResourceCache, error) {
	return resourceCache.findOrCreate(logger, tx, lockFactory, user, "resource_id", user.ResourceID)
}

// UseResourceConfig creates the ResourceConfig, recursively creating its
// parent ResourceConfig or BaseResourceType, and registers a "Use" for the
// given resource.
//
// A periodic check for a pipeline's resource will result in a
// UsedResourceConfig.
//
// ErrResourceConfigDisappeared may be returned if the resource config was
// found initially but was removed before we could use it.
//
// ErrResourceConfigAlreadyExists may be returned if a concurrent call resulted
// in a conflict.
//
// ErrResourceConfigParentDisappeared may be returned if the resource config's
// parent ResourceConfig or BaseResourceType was found initially but was
// removed before we could create the ResourceConfig.
//
// Each of these errors should result in the caller retrying from the start of
// the transaction.
func (user forResource) UseResourceConfig(logger lager.Logger, tx Tx, lockFactory lock.LockFactory, resourceConfig ResourceConfig) (*UsedResourceConfig, error) {
	return resourceConfig.findOrCreate(logger, tx, lockFactory, user, "resource_id", user.ResourceID)
}

type forResourceType struct {
	ResourceTypeID int
}

func ForResourceType(id int) ResourceUser {
	return forResourceType{id}
}

func (user forResourceType) UseResourceCache(logger lager.Logger, tx Tx, lockFactory lock.LockFactory, resourceCache ResourceCache) (*UsedResourceCache, error) {
	return resourceCache.findOrCreate(logger, tx, lockFactory, user, "resource_type_id", user.ResourceTypeID)
}

// FindOrCreateForResourceType creates the ResourceConfig, recursively creating
// its parent ResourceConfig or BaseResourceType, and registers a "Use" for the
// given resource type.
//
// A periodic check for a pipeline's resource type will result in a
// UsedResourceConfig.
//
// ErrResourceConfigDisappeared may be returned if the resource config was
// found initially but was removed before we could use it.
//
// ErrResourceConfigAlreadyExists may be returned if a concurrent call resulted
// in a conflict.
//
// ErrResourceConfigParentDisappeared may be returned if the resource config's
// parent ResourceConfig or BaseResourceType was found initially but was
// removed before we could create the ResourceConfig.
//
// Each of these errors should result in the caller retrying from the start of
// the transaction.
func (user forResourceType) UseResourceConfig(logger lager.Logger, tx Tx, lockFactory lock.LockFactory, resourceConfig ResourceConfig) (*UsedResourceConfig, error) {
	return resourceConfig.findOrCreate(logger, tx, lockFactory, user, "resource_type_id", user.ResourceTypeID)
}
