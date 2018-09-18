package gc

import (
	"github.com/concourse/atc/db"
)

type BuildLogRetentionCalculator interface {
	BuildLogsToRetain(db.Job) int
}

type buildLogRetentionCalculator struct {
	defaultBuildLogsToRetain uint64
	maxBuildLogsToRetain     uint64
}

func NewBuildLogRetentionCalculator(
	defaultBuildLogsToRetain uint64,
	maxBuildLogsToRetain uint64,
) BuildLogRetentionCalculator {
	return &buildLogRetentionCalculator{
		defaultBuildLogsToRetain: defaultBuildLogsToRetain,
		maxBuildLogsToRetain:     maxBuildLogsToRetain,
	}
}

func (blrc *buildLogRetentionCalculator) BuildLogsToRetain(job db.Job) int {
	// What does the job want?
	buildLogsToRetain := job.Config().BuildLogsToRetain

	// If not specified, set to default
	if buildLogsToRetain == 0 {
		buildLogsToRetain = int(blrc.defaultBuildLogsToRetain)
	}

	// If we don't have a max set, then we're done
	if blrc.maxBuildLogsToRetain == 0 {
		return buildLogsToRetain
	}

	// If we have a value set, and we're less than the max, then return
	if buildLogsToRetain > 0 && buildLogsToRetain < int(blrc.maxBuildLogsToRetain) {
		return buildLogsToRetain
	}

	// Else, return the max
	return int(blrc.maxBuildLogsToRetain)
}
