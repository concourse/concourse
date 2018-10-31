package v1

import (
	"context"

	"github.com/concourse/concourse/atc"
)

type putRequest struct {
	Source atc.Source `json:"source"`
	Params atc.Params `json:"params,omitempty"`
}

func (r *Resource) Put(
	ctx context.Context,
	ioConfig atc.IOConfig,
	src atc.Source,
	params atc.Params,
) (VersionedSource, error) {
	resourceDir := atc.ResourcesDir("put")

	var versionResult VersionResult
	err := RunScript(
		ctx,
		"/opt/resource/out",
		[]string{resourceDir},
		putRequest{
			Params: params,
			Source: src,
		},
		&versionResult,
		ioConfig.Stderr,
		true,
		r.Container,
	)
	if err != nil {
		return nil, err
	}

	return NewPutVersionedSource(versionResult, r.Container, resourceDir), nil
}
