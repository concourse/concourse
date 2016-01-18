package db

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/concourse/atc"
)

type ContainerIdentifier struct {
	WorkerName string
	// if it's a resource check container
	ResourceID int
	// if it's a step container
	BuildID int
	PlanID  atc.PlanID
}

type ContainerMetadata struct {
	WorkerName           string
	BuildID              int
	BuildName            string
	ResourceName         string
	PipelineID           int
	PipelineName         string
	JobName              string
	StepName             string
	Type                 ContainerType
	WorkingDirectory     string
	CheckType            string
	CheckSource          atc.Source
	EnvironmentVariables []string
	Attempts             []int
}

type Container struct {
	ContainerIdentifier
	ContainerMetadata

	ExpiresAt time.Time
	Handle    string
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

func AttemptsSliceFromString(attempts string) ([]int, error) {
	var convertedAttempts []int
	for _, item := range strings.Split(attempts, ",") {
		attemptInt, err := strconv.Atoi(item)
		if err != nil {
			return nil, err
		}
		convertedAttempts = append(convertedAttempts, attemptInt)
	}
	return convertedAttempts, nil
}

func AttemptsStringFromSlice(attempts []int) string {
	var convertedAttempts string
	for _, item := range attempts {
		attempt := strconv.Itoa(item)

		if convertedAttempts == "" {
			convertedAttempts = attempt
		} else {
			convertedAttempts += "," + attempt
		}
	}
	return convertedAttempts
}

const (
	ContainerTypeCheck ContainerType = "check"
	ContainerTypeGet   ContainerType = "get"
	ContainerTypePut   ContainerType = "put"
	ContainerTypeTask  ContainerType = "task"
)
