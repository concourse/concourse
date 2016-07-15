package resource

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

type inRequest struct {
	Source  atc.Source  `json:"source"`
	Params  atc.Params  `json:"params,omitempty"`
	Version atc.Version `json:"version,omitempty"`
}

func (resource *resource) GetContainerHandle() string {
	if resource.container != nil {
		return resource.container.Handle()
	}

	return ""
}

func (resource *resource) Get(volume worker.Volume, ioConfig IOConfig, source atc.Source, params atc.Params, version atc.Version) VersionedSource {
	resourceDir := ResourcesDir("get")

	vs := &getVersionedSource{
		volume:      volume,
		resourceDir: resourceDir,

		versionResult: versionResult{
			Version: version,
		},
	}

	vs.Runner = resource.runScript(
		"/opt/resource/in",
		[]string{resourceDir},
		inRequest{source, params, version},
		&vs.versionResult,
		ioConfig.Stderr,
		nil,
		nil,
		true,
	)

	return vs
}
