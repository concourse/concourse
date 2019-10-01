package resource

import (
	"context"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/runtime"
)

func (resource *resource) Check(
	ctx context.Context,
	spec runtime.ProcessSpec,
	runnable runtime.Runnable) ([]atc.Version, error) {
	var versions []atc.Version

	inputArgs, err := resource.Signature()
	if err != nil {
		return versions, err
	}
	err = runnable.RunScript(
		ctx,
		spec.Path,
		nil,
		inputArgs,
		&versions,
		nil,
		false,
	)
	if err != nil {
		return nil, err
	}

	return versions, nil
}
