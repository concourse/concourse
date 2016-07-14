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
	resourceDir := ResourcesDir("put")

	vs := &putVersionedSource{
		container:   resource.container,
		resourceDir: resourceDir,
	}

	vs.Runner = ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		return resource.runScript(
			"/opt/resource/out",
			[]string{resourceDir},
			outRequest{
				Params: params,
				Source: source,
			},
			&vs.versionResult,
			ioConfig.Stderr,
			artifactSource,
			vs,
			true,
		).Run(signals, ready)
	})

	return vs
}
