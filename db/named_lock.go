package db

import "fmt"

type NamedLock interface {
	Name() string
}

type ResourceCheckingLock string

func (namespacedResourceName ResourceCheckingLock) Name() string {
	return "checking:" + string(namespacedResourceName)
}

type PipelineSchedulingLock string

func (pipelineName PipelineSchedulingLock) Name() string {
	return "scheduling:" + string(pipelineName)
}

type BuildTrackingLock int

func (buildID BuildTrackingLock) Name() string {
	return fmt.Sprintf("tracking:%d", int(buildID))
}
