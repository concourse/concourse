package concourse

// Version is the version of Concourse. This variable is overridden at build
// time in the pipeline using ldflags.
var Version = "0.0.0-dev"

// WorkerVersion identifies compatibility between Concourse and a worker.
//
// Backwards-incompatible changes to the worker API should result in a major
// version bump.
//
// New features that are otherwise backwards-compatible should result in a
// minor version bump.
var WorkerVersion = "2.2"
