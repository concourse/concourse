package resource

import (
	"context"

	"github.com/concourse/atc"
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
) (VersionedSource, error) {
	resourceDir := ResourcesDir("put")

	vs := &putVersionedSource{
		container:   resource.container,
		resourceDir: resourceDir,
	}

	err := resource.runScript(
		ctx,
		"/opt/resource/out",
		[]string{resourceDir},
		putRequest{
			Params: params,
			Source: source,
		},
		&vs.versionResult,
		ioConfig.Stderr,
		true,
	)
	if err != nil {
		return nil, err
	}

	return vs, nil
}
