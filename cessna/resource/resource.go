package resource

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/cessna"
	"github.com/concourse/baggageclaim"
)

type BaseResourceType struct {
	RootFSPath string
	Name       string
}

type Resource struct {
	ResourceType RootFSable
	Source       atc.Source
}

type RootFSable interface {
	RootFSPathFor(logger lager.Logger, worker *cessna.Worker) (string, error)
}

func NewBaseResource(resourceType BaseResourceType, source atc.Source) Resource {
	return Resource{
		ResourceType: resourceType,
		Source:       source,
	}
}

func (r BaseResourceType) RootFSPathFor(logger lager.Logger, worker *cessna.Worker) (string, error) {
	spec := baggageclaim.VolumeSpec{
		Strategy: baggageclaim.ImportStrategy{
			Path: r.RootFSPath,
		},
		Privileged: true,
	}

	parentVolume, err := worker.BaggageClaimClient().CreateVolume(logger, spec)
	if err != nil {
		return "", err
	}

	// COW of RootFS Volume
	s := baggageclaim.VolumeSpec{
		Strategy: baggageclaim.COWStrategy{
			Parent: parentVolume,
		},
		Privileged: false,
	}

	v, err := worker.BaggageClaimClient().CreateVolume(logger, s)
	if err != nil {
		return "", err
	}
	return v.Path(), nil
}

type CheckRequest struct {
	Source  atc.Source  `json:"source"`
	Version atc.Version `json:"version"`
}

type CheckResponse []atc.Version

type InRequest struct {
	Source  atc.Source  `json:"source"`
	Params  atc.Params  `json:"params"`
	Version atc.Version `json:"version"`
}

type InResponse struct {
	Version  atc.Version         `json:"version"`
	Metadata []atc.MetadataField `json:"metadata,omitempty"`
}

type OutRequest struct {
	Source atc.Source `json:"source"`
	Params atc.Params `json:"params"`
}

type OutResponse struct {
	Version  atc.Version         `json:"version"`
	Metadata []atc.MetadataField `json:"metadata,omitempty"`
}

type NamedArtifacts map[string]baggageclaim.Volume
