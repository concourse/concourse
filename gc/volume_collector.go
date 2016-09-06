package gc

/*

1. add resource_caches table:

	| id | resource_type_volume_id | source_hash | params_hash | version |

	* resource_type_volume_id REFERENCES volumes (id) ON DELETE CASCADE
		* the referenced volume is either an imported worker-provided resource, or
			a volume for a custom resource type from a pipeline, which may itself have
			a cache_id

	* entries are added to the cache table on-the-fly by whoever creates the volume

	* entries are removed from the cache table as part of garbage collection, based
		on data from `next_build_inputs` and `image_resource_versions`

2. add worker_resource_versions table:

	| id | worker_name | type | path | version |

	* entries are deleted from this table and re-added as the worker's resource
		type versions change

3. volumes reference these two tables, along with `container_id`, like so:

	| volume -> cache_id                   | <- ON DELETE SET NULL
	| volume -> worker_resource_version_id | <- ON DELETE SET NULL
	| volume -> container_id               | <- ON DELETE SET NULL

4. copy-on-write volumes reference their parent volume plus a deleting state,
	 like so:

	| volume -> deleting (bool) |
	| volume -> parent_id       | <- ON DELETE RESTRICT
	| volume -> parent_deleting | <- ON DELETE RESTRICT

		UNIQUE (id, deleting)
		FOREIGN KEY (parent_id, parent_deleting)
		REFERENCES volumes (id, deleting)
		ON DELETE RESTRICT;

	creating a copy-on-write volume would insert with parent_id set to the ID,
	and parent_deleting set statically to `false`. if this fails it's because the
	parent volume is being destroyed.


volume gc logic becomes:

	if cache_id, worker_resource_version_id, and container_id are all NULL:

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
