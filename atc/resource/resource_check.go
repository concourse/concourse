package resource

import (
	"context"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/runtime"
)

type checkRequest struct {
	Source  atc.Source  `json:"source"`
	Version atc.Version `json:"version"`
}

func (resource *resource) Check(
	ctx context.Context,
	spec runtime.ProcessSpec,
	runnable runtime.Runnable) ([]atc.Version, error) {
	var versions []atc.Version

	err := runnable.RunScript(
		ctx,
		spec.Path,
		nil,
		checkRequest{resource.source, resource.version},
		&versions,
		nil,
		false,
	)
	if err != nil {
		return nil, err
	}

	return versions, nil
}
