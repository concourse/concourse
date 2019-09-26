package resource

import (
	"context"
	"fmt"

	"github.com/concourse/concourse/atc/runtime"
)

//type putRequest struct {
//	Source atc.Source `json:"source"`
//	Params atc.Params `json:"params,omitempty"`
//}

func (resource *resource) Put(
	ctx context.Context,
	runnable runtime.Runnable,
) (runtime.VersionResult, error) {
	resourceDir := ResourcesDir("put")

	vr := &runtime.VersionResult{}

	path := "/opt/resource/out"
	err := runnable.RunScript(
		ctx,
		path,
		[]string{resource.processSpec.Dir},
		resource.params,
		&vr,
		resource.processSpec.StderrWriter,
		true,
	)
	if err != nil {
		return runtime.VersionResult{}, err
	}
	if vr == nil {
		return runtime.VersionResult{}, fmt.Errorf("resource script (%s %s) output a null version", path, resourceDir)
	}

	return *vr, nil
}
