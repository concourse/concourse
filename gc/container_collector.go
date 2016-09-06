package gc

/*

add the following columns:

	| state enum('creating', 'created', 'destroying')                         |
	| build_id         int  REFERENCES builds (id)         ON DELETE SET NULL |
	| resource_id      int  REFERENCES resources (id)      ON DELETE SET NULL |
	| resource_type_id int  REFERENCES resource_types (id) ON DELETE SET NULL |
	| hijacked         bool NOT NULL DEFAULT false                            |


container gc logic becomes:

	if creating, ignore. (i think.)

	if any of the following apply:
		* build_id and resource_id and resource_type_id are null
		* resource is no longer active
		* resource type is no longer active
		* build is no longer running and is not the latest failed build of its job

	then, if hijacked:
	* if state is not 'destroying', set grace-time and set to 'destroying'
	* if state is 'destroying', check if worker container still exists
		* when gone, remove from db

	then, if not hijacked:
	* set state to 'destroying' if not already
	* destroy from worker
	* destroy from db

twists:

	* create container with grace time in case we die before recording its handle
		* grace time duration doesn't matter much; normally we immediately set
			handle, but who knows if we get killed during creation, which could
			theoretically take some time and finish after we've d/ced
	* remove grace time after saving its handle

*/
