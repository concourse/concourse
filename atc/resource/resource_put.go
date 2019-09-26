package resource

import (
	"context"
	"fmt"

	"github.com/concourse/concourse/atc/runtime"
)

func (resource *resource) Put(
	ctx context.Context,
	spec runtime.ProcessSpec,
	runnable runtime.Runnable,
) (runtime.VersionResult, error) {
	vr := &runtime.VersionResult{}

	err := runnable.RunScript(
		ctx,
		spec.Path,
		[]string{spec.Dir},
		resource,
		&vr,
		spec.StderrWriter,
		true,
	)
	if err != nil {
		return runtime.VersionResult{}, err
	}
	if vr == nil {
		return runtime.VersionResult{}, fmt.Errorf("resource script (%s %s) output a null version", spec.Path, spec.Dir)
	}

	return *vr, nil
}
