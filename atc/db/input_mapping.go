package db

import (
	"fmt"

	"github.com/concourse/concourse/atc"
)

type ResolutionFailure string

const (
	LatestVersionNotFound ResolutionFailure = "latest version of resource not found"
	VersionNotFound       ResolutionFailure = "version of resource not found"
	NoSatisfiableBuilds   ResolutionFailure = "no satisfiable builds from passed jobs found for set of inputs"
)

type PinnedVersionNotFound struct {
	PinnedVersion atc.Version
}

func (p PinnedVersionNotFound) String() ResolutionFailure {
	var text string
	for k, v := range p.PinnedVersion {
		text += fmt.Sprintf(" %s:%s", k, v)
	}
	return ResolutionFailure(fmt.Sprintf("pinned version%s not found", text))
}

type JobSet map[int]bool

type InputMapping map[string]InputResult

type InputResult struct {
	Input          *AlgorithmInput
	PassedBuildIDs []int
	ResolveError   ResolutionFailure
}

type ResourceVersion string

type AlgorithmVersion struct {
	ResourceID int
	Version    ResourceVersion
}

type AlgorithmInput struct {
	AlgorithmVersion
	FirstOccurrence bool
}

type AlgorithmOutput struct {
	AlgorithmVersion
	InputName string
}
