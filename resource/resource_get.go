package resource

import (
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

type getRequest struct {
	Source  atc.Source  `json:"source"`
	Params  atc.Params  `json:"params,omitempty"`
	Version atc.Version `json:"version,omitempty"`
}

func (resource *resource) Get(
	volume worker.Volume,
	ioConfig IOConfig,
	source atc.Source,
	params atc.Params,
	version atc.Version,
	signals <-chan os.Signal,
	ready chan<- struct{},
) (VersionedSource, error) {
	var vr versionResult

	runner := resource.runScript(
		"/opt/resource/in",
		[]string{ResourcesDir("get")},
		getRequest{source, params, version},
		&vr,
		ioConfig.Stderr,
		true,
	)

	err := runner.Run(signals, ready)
	if err != nil {
		return nil, err
	}

	return NewGetVersionedSource(volume, vr.Version, vr.Metadata), nil
}
