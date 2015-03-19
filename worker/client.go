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
	LookupContainer(Identifier) (Container, error)
}

//go:generate counterfeiter . Container

type Container interface {
	garden.Container

	Destroy() error

	Release()
}

type Identifier struct {
	Name string

	BuildID int

	Type ContainerType

	StepLocation []uint

	CheckType   string
	CheckSource atc.Source
}

const propertyPrefix = "concourse:"

func (id Identifier) gardenProperties() garden.Properties {
	props := garden.Properties{}

	if id.Name != "" {
		props[propertyPrefix+"name"] = id.Name
	}

	if id.BuildID != 0 {
		props[propertyPrefix+"build-id"] = strconv.Itoa(id.BuildID)
	}

	if id.Type != "" {
		props[propertyPrefix+"type"] = string(id.Type)
	}

	if id.StepLocation != nil {
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
