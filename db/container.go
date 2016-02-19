package db

import (
	"fmt"

	"github.com/concourse/atc"
)

type ContainerIdentifier struct {
	// if it's a resource check container
	ResourceID  int
	CheckType   string
	CheckSource atc.Source

	// if it's a step container
	BuildID int
	PlanID  atc.PlanID

	// for the check + get stages of a container with a resource backed image
	ImageResourceType   string
	ImageResourceSource atc.Source

	Stage ContainerStage
}

// ContainerStage is used to distinguish between the 3 potential containers in use by a
// step, as we'll need to run a 'check' and 'get' for the image used by the
// container, which themselves correspond to containers.
type ContainerStage string

const (
	ContainerStageCheck = "check"
	ContainerStageGet   = "get"
	ContainerStageRun   = "run"
)

type ContainerMetadata struct {
	Handle               string
	WorkerName           string
	BuildName            string
	ResourceName         string
	PipelineID           int
	PipelineName         string
	JobName              string
	StepName             string
	Type                 ContainerType
	WorkingDirectory     string
	EnvironmentVariables []string
	Attempts             []int
	User                 string
}

type Container struct {
	ContainerIdentifier
	ContainerMetadata
}

type ContainerType string

func (containerType ContainerType) String() string {
	return string(containerType)
}

func ContainerTypeFromString(containerType string) (ContainerType, error) {
	switch containerType {
	case "check":
		return ContainerTypeCheck, nil
	case "get":
		return ContainerTypeGet, nil
	case "put":
		return ContainerTypePut, nil
	case "task":
		return ContainerTypeTask, nil
	default:
		return "", fmt.Errorf("Unrecognized containerType: %s", containerType)
	}
}

const (
	ContainerTypeCheck ContainerType = "check"
	ContainerTypeGet   ContainerType = "get"
	ContainerTypePut   ContainerType = "put"
	ContainerTypeTask  ContainerType = "task"
)
