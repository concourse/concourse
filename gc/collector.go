package gc

// GC in order of dependencies, so that we can do things in one pass:
//
// * workers
// * containers
// * caches
// * volumes
// * db
