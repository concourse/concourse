package gc

/*

1. resource_configs table:
	| id | resource_cache_id | base_resource_type_id | source_hash | params_hash |

	container -> resource_config_id ON DELETE SET NULL

	resources -> resource_config_id
	resource_types -> resource_config_id

	gc resource_configs when not referenced by any active resources or resource types

1. add resource_caches table:

	| id | resource_config_id | version |

	* resource_cache_id REFERENCES resource_caches (id) ON DELETE CASCADE

	* base_resource_type_id REFERENCES base_resource_types (id) ON DELETE CASCADE

	* entries are added to the cache table on-the-fly by whoever creates the volume

	* entries are removed from the cache table as part of garbage collection, based
		on data from `next_build_inputs` and `image_resource_versions`

2. add resource_cache_uses table:

	| resource_cache_id | build_id | resource_id | resource_type_id |

	* ON DELETE RESTRICT for cache_id (can't delete from resource_caches when there is resource_cache_uses)

2. add worker_base_resource_types table:

	| worker_name | base_resource_type_id |

	* entries are deleted from this table and re-added as the worker's resource
		type versions change

3. volumes reference these two tables, along with `container_id`, like so:

	| volume -> cache_id              | <- ON DELETE SET NULL
	| volume -> base_resource_type_id | <- ON DELETE SET NULL (when base_resource_type is deleted base_resource_type_id is set to NULL)
	| volume -> container_id          | <- ON DELETE SET NULL

4. copy-on-write volumes reference their parent volume plus a deleting state,
	 like so:

	'creating': exists in db only
	'created': exists in db and on worker
	'initializing': doing stuff populate it (resource fetch, import data, etc)
	'initialized': ready for use
	'destroying': removing from worker
	(gone): gone from worker and db

	| volume -> state enum('creating', 'created', 'initializing', 'initialized', 'destroying') |
	| volume -> parent_id       | <- ON DELETE RESTRICT
	| volume -> parent_state | <- ON DELETE RESTRICT

		UNIQUE (id, state)
		FOREIGN KEY (parent_id, parent_state)
		REFERENCES volumes (id, state)
		ON DELETE RESTRICT;

	creating a copy-on-write volume would insert with parent_id set to the ID,
	and parent_state set statically to `initialized`. if this fails it's because
	the parent volume is being destroyed.


volume gc logic becomes:

	if cache_id, worker_base_resource_type_id, and container_id are all NULL:

	* mark the volume as 'deleting'
		* if that fails, log and bail; a child volume must be using it
	* delete the volume from the worker
		* if that fails, log and bail; either something is broken on the worker or
		  the network flaked out
	* delete the volume from the database
		* if that fails, log; this should only fail due to network errors or
		  something


this allows:

* nuking a cache that is "bad" (even though ugh this should never happen but ugh)
* invalidating caches when a new resource type is added
* ensuring caches are for the correct resource type (i.e. invalidate when new
  version comes out, don't reuse across pipelines)
* more control over which things are cached
* consolidates caching logic


hard bits:

* solved: race with destroying parent volumes (solved by 'deleting' reference)
* unsolved: race with creating parent and child volumes (i.e. create parent, upsert cache, start populating volume, cache removed, parent removed, create child -> boom)
	* ensure parent and child created within single transaction?
* is all of this reentrant?

*/
