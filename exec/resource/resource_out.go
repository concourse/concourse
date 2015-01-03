package resource

import (
	"os"

	"github.com/concourse/atc"
	"github.com/tedsuo/ifrit"
)

type outRequest struct {
	Source atc.Source `json:"source"`
	Params atc.Params `json:"params,omitempty"`
}

func (resource *resource) Put(ioConfig IOConfig, source atc.Source, params atc.Params, artifactSource ArtifactSource) VersionedSource {
	vs := &versionedSource{
		container: resource.container,
	}

	vs.Runner = ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		err := artifactSource.StreamTo(vs)
		if err != nil {
			return err
		}

		return resource.runScript(
			"/opt/resource/out",
			[]string{ResourcesDir},
			outRequest{
				Params: params,
				Source: source,
			},
			&vs.versionResult,
			ioConfig.Stderr,
		).Run(signals, ready)
	})

	return vs
}
