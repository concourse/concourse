package resource

import "github.com/concourse/atc"

type inRequest struct {
	Source  atc.Source  `json:"source"`
	Params  atc.Params  `json:"params,omitempty"`
	Version atc.Version `json:"version,omitempty"`
}

func (resource *resource) Get(ioConfig IOConfig, source atc.Source, params atc.Params, version atc.Version) VersionedSource {
	resourceDir := ResourcesDir("get")

	vs := &versionedSource{
		container:   resource.container,
		resourceDir: resourceDir,
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
		"get",
	)

	return vs
}
