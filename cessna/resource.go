package cessna

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/baggageclaim"
	uuid "github.com/nu7hatch/gouuid"
)

type BaseResourceType struct {
	RootFSPath string
	Name       string
}

type Resource struct {
	ResourceType RootFSable
	Source       atc.Source
}

//go:generate counterfeiter . RootFSable
type RootFSable interface {
	RootFSPathFor(logger lager.Logger, worker Worker) (string, error)
}

func NewBaseResource(resourceType BaseResourceType, source atc.Source) Resource {
	return Resource{
		ResourceType: resourceType,
		Source:       source,
	}
}

func (r BaseResourceType) RootFSPathFor(logger lager.Logger, worker Worker) (string, error) {
	spec := baggageclaim.VolumeSpec{
		Strategy: baggageclaim.ImportStrategy{
			Path: r.RootFSPath,
		},
		Privileged: true,
	}

	handle, err := uuid.NewV4()
	if err != nil {
		return "", err
	}

	parentVolume, err := worker.BaggageClaimClient().CreateVolume(logger, handle.String(), spec)
	if err != nil {
		return "", err
	}

	// COW of RootFS Volume
	s := baggageclaim.VolumeSpec{
		Strategy: baggageclaim.COWStrategy{
			Parent: parentVolume,
		},
		Privileged: true,
	}

	handle, err = uuid.NewV4()
	if err != nil {
		return "", err
	}

	v, err := worker.BaggageClaimClient().CreateVolume(logger, handle.String(), s)
	if err != nil {
		return "", err
	}
	return "raw://" + v.Path(), nil
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
