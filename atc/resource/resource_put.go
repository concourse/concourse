package resource

import (
	"context"
	"fmt"

	"github.com/concourse/concourse/atc/runtime"

	"github.com/concourse/concourse/atc"
)

//type putRequest struct {
//	Source atc.Source `json:"source"`
//	Params atc.Params `json:"params,omitempty"`
//}

func (resource *resource) Put(
	ctx context.Context,
	ioConfig runtime.IOConfig,
	source atc.Source,
	params atc.Params,
) (runtime.VersionResult, error) {
	resourceDir := ResourcesDir("put")

	vr := &runtime.VersionResult{}

	path := "/opt/resource/out"
	err := resource.runScript(
		ctx,
		path,
		[]string{resourceDir},
		runtime.PutRequest{
			Params: params,
			Source: source,
		},
		&vr,
		ioConfig.Stderr,
		true,
	)
	if err != nil {
		return runtime.VersionResult{}, err
	}
	if vr == nil {
		return VersionResult{}, fmt.Errorf("resource script (%s %s) output a null version", path, resourceDir)
	}

	return *vr, nil
}
