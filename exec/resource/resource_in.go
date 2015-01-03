package resource

import "github.com/concourse/atc"

type inRequest struct {
	Source  atc.Source  `json:"source"`
	Params  atc.Params  `json:"params,omitempty"`
	Version atc.Version `json:"version,omitempty"`
}

func (resource *resource) Get(source atc.Source, params atc.Params, version atc.Version) VersionedSource {
	vs := &versionedSource{
		container: resource.container,
	}

	vs.Runner = resource.runScript(
		"/opt/resource/in",
		[]string{ResourcesDir},
		inRequest{source, params, version},
		&vs.versionResult,
	)

	return vs
}
