package resource

import (
	"context"
	"fmt"

	"github.com/concourse/concourse/atc/runtime"
)

func (resource *resource) Put(
	ctx context.Context,
	spec runtime.ProcessSpec,
	runnable runtime.Runner,
) (runtime.VersionResult, error) {
	vr := runtime.VersionResult{}

	inputArgs, err := resource.Signature()
	if err != nil {
		return vr, err
	}

	err = runnable.RunScript(
		ctx,
		spec.Path,
		[]string{spec.Dir},
		inputArgs,
		&vr,
		spec.StderrWriter,
		true,
	)
	if err != nil {
		return runtime.VersionResult{}, err
	}
	if vr.Version == nil {
		return runtime.VersionResult{}, fmt.Errorf("resource script (%s %s) output a null version", spec.Path, spec.Dir)
	}

	return vr, nil
}
