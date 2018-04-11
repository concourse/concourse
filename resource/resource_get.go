package resource

import (
	"context"

	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

type getRequest struct {
	Source  atc.Source  `json:"source"`
	Params  atc.Params  `json:"params,omitempty"`
	Version atc.Version `json:"version,omitempty"`
}

func (resource *resource) Get(
	ctx context.Context,
	volume worker.Volume,
	ioConfig IOConfig,
	source atc.Source,
	params atc.Params,
	version atc.Version,
) (VersionedSource, error) {
	var vr versionResult

	err := resource.runScript(
		ctx,
		"/opt/resource/in",
		[]string{ResourcesDir("get")},
		getRequest{source, params, version},
		&vr,
		ioConfig.Stderr,
		true,
	)
	if err != nil {
		return nil, err
	}

	return NewGetVersionedSource(volume, vr.Version, vr.Metadata), nil
}
