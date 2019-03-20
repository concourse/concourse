package gc

import (
	"github.com/concourse/concourse/atc/db"
)

type BuildLogRetentionCalculator interface {
	BuildLogsToRetain(db.Job) (int, int)
}

type buildLogRetentionCalculator struct {
	defaultBuildLogsToRetain uint64
	maxBuildLogsToRetain uint64
	defaultBuildLogsDaysToRetain uint64
	maxBuildLogsDaysToRetain uint64
}

func NewBuildLogRetentionCalculator(
	defaultBuildLogsToRetain uint64,
	maxBuildLogsToRetain uint64,
	defaultBuildLogsDaysToRetain uint64,
	maxBuildLogsDaysToRetain uint64,
) BuildLogRetentionCalculator {
	return &buildLogRetentionCalculator{
		defaultBuildLogsToRetain: defaultBuildLogsToRetain,
		maxBuildLogsToRetain: maxBuildLogsToRetain,
		defaultBuildLogsDaysToRetain: defaultBuildLogsDaysToRetain,
		maxBuildLogsDaysToRetain: maxBuildLogsDaysToRetain,
	}
}

func (blrc *buildLogRetentionCalculator) BuildLogsToRetain(job db.Job) (int, int) {
	// What does the job want?
	var daysToRetainBuildLogs = 0
	var buildLogsToRetain = 0
	if job.Config().BuildLogRetention != nil {
		daysToRetainBuildLogs = job.Config().BuildLogRetention.Days
		buildLogsToRetain = job.Config().BuildLogRetention.Builds
	} else {
		buildLogsToRetain = job.Config().BuildLogsToRetain
	}

	// If not specified, set to default
	if buildLogsToRetain == 0 {
		buildLogsToRetain = int(blrc.defaultBuildLogsToRetain)
	}
	if daysToRetainBuildLogs == 0 {
		daysToRetainBuildLogs = int(blrc.defaultBuildLogsDaysToRetain)
	}

	// If we don't have a max set, then we're done
	if blrc.maxBuildLogsToRetain == 0 && blrc.maxBuildLogsDaysToRetain == 0 {
		return buildLogsToRetain, daysToRetainBuildLogs
	}

	var buildLogsToRetainReturn int
	var buildLogsDaysToRetainReturn int
	// If we have a value set, and we're less than the max, then return
	if buildLogsToRetain > 0 && buildLogsToRetain < int(blrc.maxBuildLogsToRetain) {
		buildLogsToRetainReturn = buildLogsToRetain
	} else {
		buildLogsToRetainReturn= int(blrc.maxBuildLogsToRetain)
	}

	if daysToRetainBuildLogs > 0 && daysToRetainBuildLogs < int(blrc.maxBuildLogsDaysToRetain) {
		buildLogsDaysToRetainReturn = daysToRetainBuildLogs
	} else {
		buildLogsDaysToRetainReturn = int(blrc.maxBuildLogsDaysToRetain)
	}

	return buildLogsToRetainReturn, buildLogsDaysToRetainReturn

}
