package db

// ResourceCacheUser designates the column to set in the resource_cache_users
// table.
type ResourceCacheUser interface {
	SQLMap() map[string]interface{}
}

type forBuild struct {
	BuildID int
}

func ForBuild(id int) ResourceCacheUser {
	return forBuild{id}
}

func (user forBuild) SQLMap() map[string]interface{} {
	return map[string]interface{}{
		"build_id": user.BuildID,
	}
}

type forContainer struct {
	ContainerID int
}

func ForContainer(id int) ResourceCacheUser {
	return forContainer{id}
}

func (user forContainer) SQLMap() map[string]interface{} {
	return map[string]interface{}{
		"container_id": user.ContainerID,
	}
}
