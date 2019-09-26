package resource

import (
	"context"

	"github.com/concourse/concourse/atc/runtime"

	"github.com/concourse/concourse/atc"
)

type checkRequest struct {
	Source  atc.Source  `json:"source"`
	Version atc.Version `json:"version"`
}

func (resource *resource) Check(ctx context.Context, runnable runtime.Runnable) ([]atc.Version, error) {
	var versions []atc.Version

	err := runnable.RunScript(
		ctx,
		resource.processSpec.Path,
		nil,
		resource.params,
		&versions,
		nil,
		false,
	)
	if err != nil {
		return nil, err
	}

	return versions, nil
}
