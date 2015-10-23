package db

import (
	"fmt"
	"time"

	"github.com/concourse/atc"
)

type ContainerIdentifier struct {
	Name         string
	PipelineName string
	BuildID      int
	Type         ContainerType
	WorkerName   string
	CheckType    string
	CheckSource  atc.Source
	StepLocation uint
}

type Container struct {
	ContainerIdentifier

	ExpiresAt time.Time
	Handle    string
}

type ContainerType string

func (containerType ContainerType) String() string {
	return string(containerType)
}

func containerTypeFromString(containerType string) (ContainerType, error) {
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
