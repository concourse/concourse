package worker

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/baggageclaim"
)

//go:generate counterfeiter . Client

type Client interface {
	CreateContainer(Identifier, ContainerSpec) (Container, error)
	FindContainerForIdentifier(Identifier) (Container, bool, error)
	LookupContainer(handle string) (Container, bool, error)

	Satisfying(WorkerSpec) (Worker, error)
}

//go:generate counterfeiter . Container

type Container interface {
	garden.Container

	Destroy() error

	Release()

	IdentifierFromProperties() Identifier

	Volumes() ([]baggageclaim.Volume, error)
}

type Identifier struct {
	Name string

	PipelineName string

	BuildID int

	Type db.ContainerType

	StepLocation uint

	CheckType   string
	CheckSource atc.Source

	WorkerName string
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

type MultipleWorkersFoundContainerError struct {
	Names []string
}

func (err MultipleWorkersFoundContainerError) Error() string {
	return fmt.Sprintf("multiple workers found specified container, expected one: %s", strings.Join(err.Names, ", "))
}

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
