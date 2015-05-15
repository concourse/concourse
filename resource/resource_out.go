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

func (resource *resource) Put(ioConfig IOConfig, source atc.Source, params atc.Params, getParams atc.Params, artifactSource ArtifactSource) VersionedSource {
	putDir := ResourcesDir("put")
	getDir := ResourcesDir("get")

	putVersionedSource := &versionedSource{
		container:   resource.container,
		resourceDir: putDir,
	}

	putVersionedSource.Runner = ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		err := resource.runScript(
			"/opt/resource/out",
			[]string{putDir},
			outRequest{
				Params: params,
				Source: source,
			},
			&putVersionedSource.versionResult,
			ioConfig.Stderr,
			artifactSource,
			putVersionedSource,
			true,
			"put",
		).Run(signals, ready)
		if err != nil {
			return err
		}

		getVersionedSource := &versionedSource{
			container:   resource.container,
			resourceDir: getDir,
		}

		err = resource.runScript(
			"/opt/resource/in",
			[]string{getDir},
			inRequest{
				Source:  source,
				Params:  getParams,
				Version: putVersionedSource.Version(),
			},
			&getVersionedSource.versionResult,
			ioConfig.Stderr,
			nil,
			nil,
			true,
			"get",
		).Run(signals, make(chan struct{}))
		if err != nil {
			return err
		}

		putVersionedSource.resourceDir = getDir

		return nil
	})

	return putVersionedSource
}
