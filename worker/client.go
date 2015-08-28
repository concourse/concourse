package worker

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc"
)

//go:generate counterfeiter . Client

type Client interface {
	CreateContainer(Identifier, ContainerSpec) (Container, error)
	FindContainerForIdentifier(Identifier) (Container, error)
	FindContainersForIdentifier(Identifier) ([]Container, error)

	// LookupContainer performs a lookup for a container with the provided handle.
	// Returns error and nil Container if no container is found for the provided handle.
	LookupContainer(string) (Container, error)

	Name() string
}

//go:generate counterfeiter . Container

type Container interface {
	garden.Container

	Destroy() error

	Release()

	IdentifierFromProperties() (Identifier, error)
}

type Identifier struct {
	Name string

	PipelineName string

	BuildID int

	Type ContainerType

	StepLocation uint

	CheckType   string
	CheckSource atc.Source
}

const propertyPrefix = "concourse:"

func (id Identifier) gardenProperties() garden.Properties {
	props := garden.Properties{}

	if id.Name != "" {
		props[propertyPrefix+"name"] = id.Name
	}

	if id.PipelineName != "" {
		props[propertyPrefix+"pipeline-name"] = id.PipelineName
	}

	if id.BuildID != 0 {
		props[propertyPrefix+"build-id"] = strconv.Itoa(id.BuildID)
	}

	if id.Type != "" {
		props[propertyPrefix+"type"] = string(id.Type)
	}

	if id.StepLocation != 0 {
		props[propertyPrefix+"location"] = fmt.Sprintf("%v", id.StepLocation)
	}

	if id.CheckType != "" {
		props[propertyPrefix+"check-type"] = id.CheckType
	}

	if id.CheckSource != nil {
		payload, _ := json.Marshal(id.CheckSource) // shhhh
		props[propertyPrefix+"check-source"] = string(payload)
	}

	return props
}

type ContainerType string

const (
	ContainerTypeCheck ContainerType = "check"
	ContainerTypeGet   ContainerType = "get"
	ContainerTypePut   ContainerType = "put"
	ContainerTypeTask  ContainerType = "task"
)

type MultipleContainersError struct {
	Handles []string
}

func (err MultipleContainersError) Error() string {
	return fmt.Sprintf("multiple containers found, expected one: %s", strings.Join(err.Handles, ", "))
}

type MultiWorkerError struct {
	workerErrors map[string]error
}

func (mwe *MultiWorkerError) AddError(workerName string, err error) {
	if mwe.workerErrors == nil {
		mwe.workerErrors = map[string]error{}
	}

	mwe.workerErrors[workerName] = err
}

func (mwe MultiWorkerError) Errors() map[string]error {
	return mwe.workerErrors
}

func (err MultiWorkerError) Error() string {
	errorMessage := ""
	if err.workerErrors != nil {
		for workerName, err := range err.workerErrors {
			errorMessage = fmt.Sprintf("%s workerName: %s, error: %s", errorMessage, workerName, err)
		}
	}
	return errorMessage
}
