package v1

import (
	"context"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/worker"
)

type getRequest struct {
	Source  atc.Source  `json:"source"`
	Params  atc.Params  `json:"params,omitempty"`
	Version atc.Version `json:"version,omitempty"`
}

func (r *Resource) Get(
	ctx context.Context,
	volume worker.Volume,
	ioConfig atc.IOConfig,
	src atc.Source,
	params atc.Params,
	version atc.Version,
) (VersionedSource, error) {
	var vr VersionResult

	err := RunScript(
		ctx,
		"/opt/resource/in",
		[]string{atc.ResourcesDir("get")},
		getRequest{src, params, version},
		&vr,
		ioConfig.Stderr,
		true,
		r.Container,
	)
	if err != nil {
		return nil, err
	}

	return NewGetVersionedSource(volume, vr.Version, vr.Metadata), nil
}
