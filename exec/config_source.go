package exec

import (
	"github.com/cloudfoundry-incubator/candiedyaml"
	"github.com/concourse/atc"
)

//go:generate counterfeiter . TaskConfigSource
type TaskConfigSource interface {
	FetchConfig(ArtifactSource) (atc.TaskConfig, error)
}

type StaticConfigSource struct {
	Config atc.TaskConfig
}

func (configSource StaticConfigSource) FetchConfig(ArtifactSource) (atc.TaskConfig, error) {
	return configSource.Config, nil
}

type FileConfigSource struct {
	Path string
}

func (configSource FileConfigSource) FetchConfig(source ArtifactSource) (atc.TaskConfig, error) {
	stream, err := source.StreamFile(configSource.Path)
	if err != nil {
		return atc.TaskConfig{}, err
	}

	defer stream.Close()

	var config atc.TaskConfig
	err = candiedyaml.NewDecoder(stream).Decode(&config)
	if err != nil {
		return atc.TaskConfig{}, err
	}

	err = config.Validate()
	if err != nil {
		return atc.TaskConfig{}, err
	}

	return config, nil
}

type MergedConfigSource struct {
	A TaskConfigSource
	B TaskConfigSource
}

func (configSource MergedConfigSource) FetchConfig(source ArtifactSource) (atc.TaskConfig, error) {
	aConfig, err := configSource.A.FetchConfig(source)
	if err != nil {
		return atc.TaskConfig{}, err
	}

	bConfig, err := configSource.B.FetchConfig(source)
	if err != nil {
		return atc.TaskConfig{}, err
	}

	return aConfig.Merge(bConfig), nil
}
