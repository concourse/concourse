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

	var logRetention atc.BuildLogRetention

	// If we don't have a max set, then we're done
	if blrc.maxBuildLogsToRetain == 0 && blrc.maxDaysToRetainBuildLogs == 0 {
		logRetention.Builds = buildLogsToRetain
		logRetention.MinimumSucceededBuilds = minSuccessBuildLogsToRetain
		logRetention.Days = daysToRetainBuildLogs
		return logRetention
	}

	logRetention.Builds = int(blrc.maxBuildLogsToRetain)
	logRetention.Days = int(blrc.maxDaysToRetainBuildLogs)

	if logRetention.Builds > 0 {
		// current value will be the max, only override if it's less than the current value
		if buildLogsToRetain > 0 && (buildLogsToRetain < logRetention.Builds) {
			logRetention.Builds = buildLogsToRetain
		}
	} else {
		logRetention.Builds = buildLogsToRetain
	}

	if logRetention.Days > 0 {
		// current value will be the max, only override if it's less than the current value
		if daysToRetainBuildLogs > 0 && daysToRetainBuildLogs < logRetention.Days {
			logRetention.Days = daysToRetainBuildLogs
		}
	} else {
		logRetention.Days = daysToRetainBuildLogs
	}

	// successBuildLogsToRetain defaults to 0, and up to buildLogsToRetain.
	if minSuccessBuildLogsToRetain >= 0 && minSuccessBuildLogsToRetain <= logRetention.Builds {
		logRetention.MinimumSucceededBuilds = minSuccessBuildLogsToRetain
	}

	return logRetention

}
