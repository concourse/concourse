package resource

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
)

type inRequest struct {
	Source  atc.Source  `json:"source"`
	Params  atc.Params  `json:"params,omitempty"`
	Version atc.Version `json:"version,omitempty"`
}

func (resource *resource) Get(container worker.Container, ioConfig IOConfig, source atc.Source, params atc.Params, version atc.Version, logger lager.Logger) VersionedSource {
	resourceDir := ResourcesDir("get")

	var vContainer worker.Container
	if container != nil {
		vContainer = container
		resource.container = vContainer
	} else {
		vContainer = resource.container
	}

	if logger != nil {
		if container == nil {
			logger.Info("resource-get-container-is-nil!")
		} else {
			logger.Info("resource-get", lager.Data{"container": container.Handle()})
		}

		if resource.container != nil {
			logger.Info("resource-get", lager.Data{"resource-container": resource.container.Handle()})
		} else {
			logger.Info("resource-get-resource-container-is-nil!")
		}
	}

	vs := &versionedSource{
		container:   vContainer,
		resourceDir: resourceDir,

		versionResult: versionResult{
			Version: version,
		},
	}

	if logger != nil {
		logger.Debug("get-runner-start")
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

	if logger != nil {
		logger.Debug("get-runner-done")
	}

	return vs
}
