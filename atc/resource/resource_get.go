package resource

import (
	"context"

	"github.com/concourse/concourse/atc/runtime"
)

func (resource *resource) Get(
	ctx context.Context,
	spec runtime.ProcessSpec,
	runnable runtime.Runner,
) (runtime.VersionResult, error) {
	var vr runtime.VersionResult

	inputArgs, err := resource.Signature()
	if err != nil {
		return vr, err
	}

	err = runnable.RunScript(
		ctx,
		spec.Path,
		spec.Args,
		inputArgs,
		&vr,
		spec.StderrWriter,
		true,
	)
	if err != nil {
		return runtime.VersionResult{}, err
	}

	return vr, nil
}
