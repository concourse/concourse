package db

import "fmt"

type BuildPreparationStatus string

const (
	BuildPreparationStatusUnknown     BuildPreparationStatus = "unknown"
	BuildPreparationStatusBlocking    BuildPreparationStatus = "blocking"
	BuildPreparationStatusNotBlocking BuildPreparationStatus = "not_blocking"
)

type MissingInputReasons map[string]string

const (
	NoVerionsSatisfiedPassedConstraints string = "no versions satisfy passed constraints"
	NoVersionsAvailable                 string = "no versions available"
	PinnedVersionUnavailable            string = "pinned version %s is not available"
)

func (mir MissingInputReasons) RegisterPassedConstraint(inputName string) {
	mir[inputName] = NoVerionsSatisfiedPassedConstraints
}

func (mir MissingInputReasons) RegisterNoVersions(inputName string) {
	mir[inputName] = NoVersionsAvailable
}

func (mir MissingInputReasons) RegisterPinnedVersionUnavailable(inputName string, version string) {
	mir[inputName] = fmt.Sprintf(PinnedVersionUnavailable, version)
}

func (mir MissingInputReasons) Append(otherReasons MissingInputReasons) {
	for k, v := range otherReasons {
		mir[k] = v
	}
}

type BuildPreparation struct {
	BuildID             int
	PausedPipeline      BuildPreparationStatus
	PausedJob           BuildPreparationStatus
	MaxRunningBuilds    BuildPreparationStatus
	Inputs              map[string]BuildPreparationStatus
	InputsSatisfied     BuildPreparationStatus
	MissingInputReasons MissingInputReasons
}

func NewBuildPreparation(buildID int) BuildPreparation {
	return BuildPreparation{
		BuildID:             buildID,
		PausedPipeline:      BuildPreparationStatusUnknown,
		PausedJob:           BuildPreparationStatusUnknown,
		MaxRunningBuilds:    BuildPreparationStatusUnknown,
		Inputs:              map[string]BuildPreparationStatus{},
		InputsSatisfied:     BuildPreparationStatusUnknown,
		MissingInputReasons: MissingInputReasons{},
	}
}
