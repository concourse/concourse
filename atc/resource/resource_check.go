package resource

import (
	"context"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/runtime"
)

func (resource *resource) Check(
	ctx context.Context,
	spec runtime.ProcessSpec,
	runnable runtime.Runner) ([]atc.Version, error) {
	var versions []atc.Version

	input, err := resource.Signature()
	if err != nil {
		return versions, err
	}
	err = runnable.RunScript(
		ctx,
		spec.Path,
		nil,
		input,
		&versions,
		nil,
		false,
	)
	return versions, err
}
