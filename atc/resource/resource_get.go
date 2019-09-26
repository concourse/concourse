package resource

import (
	"context"

	"github.com/concourse/concourse/atc/runtime"

	"github.com/concourse/concourse/atc"
)

type getRequest struct {
	Source  atc.Source  `json:"source"`
	Params  atc.Params  `json:"params,omitempty"`
	Version atc.Version `json:"version,omitempty"`
}

func (resource *resource) Get(
	ctx context.Context,
	//volume worker.Volume,
	//ioConfig runtime.IOConfig,
	//source atc.Source,
	//params atc.Params,
	//version atc.Version,
	runnable runtime.Runnable,
) (runtime.VersionResult, error) {
	var vr runtime.VersionResult

	// should be something on worker client, not direct runScript call
	err := runnable.RunScript(
		ctx,
		resource.processSpec.Path,
		//[]string{ResourcesDir("get")},
		[]string{resource.processSpec.Dir},
		resource.params,
		&vr,
		resource.processSpec.StderrWriter,
		true,
	)
	if err != nil {
		return runtime.VersionResult{}, err
	}

	return vr, nil
}
