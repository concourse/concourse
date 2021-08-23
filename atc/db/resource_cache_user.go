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

type forInMemoryBuild struct {
	BuildID int
	CreateTime int
}

func ForInMemoryBuild(id int, createTime int) ResourceCacheUser {
	return forInMemoryBuild{id, createTime}
}

func (user forInMemoryBuild) SQLMap() map[string]interface{} {
	return map[string]interface{}{
		"in_memory_build_id": user.BuildID,
		"in_memory_build_create_time": user.CreateTime,
	}
}
