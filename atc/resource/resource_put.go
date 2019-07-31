package resource

import (
	"context"

	"github.com/concourse/concourse/atc"
)

type putRequest struct {
	Source atc.Source `json:"source"`
	Params atc.Params `json:"params,omitempty"`
}

func (resource *resource) Put(
	ctx context.Context,
	ioConfig IOConfig,
	source atc.Source,
	params atc.Params,
) (VersionResult, error) {
	resourceDir := ResourcesDir("put")

	vr := &VersionResult{}

	err := resource.runScript(
		ctx,
		"/opt/resource/out",
		[]string{resourceDir},
		putRequest{
			Params: params,
			Source: source,
		},
		&vr,
		ioConfig.Stderr,
		true,
	)
	if err != nil {
		return VersionResult{}, err
	}

	return *vr, nil
}
