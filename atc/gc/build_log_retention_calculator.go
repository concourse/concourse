package gc

import (
	"github.com/concourse/concourse/atc"
)

type BuildLogRetentionCalculator interface {
	BuildLogsToRetain(atc.JobConfig) atc.BuildLogRetention
}

type buildLogRetentionCalculator struct {
	defaultBuildLogsToRetain     uint64
	maxBuildLogsToRetain         uint64
	defaultDaysToRetainBuildLogs uint64
	maxDaysToRetainBuildLogs     uint64
}

func NewBuildLogRetentionCalculator(
	defaultBuildLogsToRetain uint64,
	maxBuildLogsToRetain uint64,
	defaultDaysToRetainBuildLogs uint64,
	maxDaysToRetainBuildLogs uint64,
) BuildLogRetentionCalculator {
	return &buildLogRetentionCalculator{
		defaultBuildLogsToRetain:     defaultBuildLogsToRetain,
		maxBuildLogsToRetain:         maxBuildLogsToRetain,
		defaultDaysToRetainBuildLogs: defaultDaysToRetainBuildLogs,
		maxDaysToRetainBuildLogs:     maxDaysToRetainBuildLogs,
	}
}

func (blrc *buildLogRetentionCalculator) BuildLogsToRetain(jobConfig atc.JobConfig) atc.BuildLogRetention {
	// What does the job want?
	var daysToRetainBuildLogs = 0
	var buildLogsToRetain = 0
	var minSuccessBuildLogsToRetain = 0
	if jobConfig.BuildLogRetention != nil {
		daysToRetainBuildLogs = jobConfig.BuildLogRetention.Days
		buildLogsToRetain = jobConfig.BuildLogRetention.Builds
		minSuccessBuildLogsToRetain = jobConfig.BuildLogRetention.MinimumSucceededBuilds
	} else {
		buildLogsToRetain = jobConfig.BuildLogsToRetain
	}

	// If not specified, set to default
	if buildLogsToRetain == 0 {
		buildLogsToRetain = int(blrc.defaultBuildLogsToRetain)
	}
	if daysToRetainBuildLogs == 0 {
		daysToRetainBuildLogs = int(blrc.defaultDaysToRetainBuildLogs)
	}

	// If we don't have a max set, then we're done
	if blrc.maxBuildLogsToRetain == 0 && blrc.maxDaysToRetainBuildLogs == 0 {
		return atc.BuildLogRetention{buildLogsToRetain, minSuccessBuildLogsToRetain, daysToRetainBuildLogs}
	}

	var logRetention atc.BuildLogRetention
	// If we have a value set, and we're less than the max, then return
	if buildLogsToRetain > 0 && buildLogsToRetain < int(blrc.maxBuildLogsToRetain) {
		logRetention.Builds = buildLogsToRetain
	} else {
		logRetention.Builds = int(blrc.maxBuildLogsToRetain)
	}

	if daysToRetainBuildLogs > 0 && daysToRetainBuildLogs < int(blrc.maxDaysToRetainBuildLogs) {
		logRetention.Days = daysToRetainBuildLogs
	} else {
		logRetention.Days = int(blrc.maxDaysToRetainBuildLogs)
	}

	// successBuildLogsToRetain defaults to 0, and up to buildLogsToRetain.
	if minSuccessBuildLogsToRetain >= 0 && minSuccessBuildLogsToRetain <= logRetention.Builds {
		logRetention.MinimumSucceededBuilds = minSuccessBuildLogsToRetain
	} else {
		logRetention.MinimumSucceededBuilds = 0
	}

	return logRetention

}
