package worker

import (
	"fmt"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc/db"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Client

type Client interface {
	CreateContainer(lager.Logger, Identifier, ContainerSpec) (Container, error)
	FindContainerForIdentifier(lager.Logger, Identifier) (Container, bool, error)
	LookupContainer(lager.Logger, string) (Container, bool, error)

	Satisfying(WorkerSpec) (Worker, error)
}

//go:generate counterfeiter . Container

type Container interface {
	garden.Container

	Destroy() error

	Release()

	Volumes() []baggageclaim.Volume
}

type Identifier db.ContainerIdentifier

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
