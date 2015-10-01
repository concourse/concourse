package db

import (
	"fmt"
	"time"

	"github.com/concourse/atc"
)

type ContainerInfo struct {
	Handle       string
	Name         string
	PipelineName string
	BuildID      int
	Type         ContainerType
	WorkerName   string
	ExpiresAt    time.Time
	CheckType    string
	CheckSource  atc.Source
}

type ContainerType string

func (containerType ContainerType) ToString() string {
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

type ContainerIdentifier struct {
	Name         string
	PipelineName string
	BuildID      int
	Type         ContainerType
	WorkerName   string
	CheckType    string
	CheckSource  atc.Source
}
