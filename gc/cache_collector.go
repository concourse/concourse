package gc

/*

* removing cache_uses for builds that have completed

* remove resource_caches if all of the following apply:
	* no cache_uses (possibly w/ db constraint)
	* not a candidate for `next_build_inputs`
	* not a `image_resource_version` used in latest build of any job

*/
