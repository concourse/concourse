package exec

import (
	"github.com/cloudfoundry-incubator/candiedyaml"
	"github.com/concourse/atc"
)

//go:generate counterfeiter . BuildConfigSource
type BuildConfigSource interface {
	FetchConfig(ArtifactSource) (atc.BuildConfig, error)
}

type StaticConfigSource struct {
	Config atc.BuildConfig
}

func (configSource StaticConfigSource) FetchConfig(ArtifactSource) (atc.BuildConfig, error) {
	return configSource.Config, nil
}

type FileConfigSource struct {
	Path string
}

func (configSource FileConfigSource) FetchConfig(source ArtifactSource) (atc.BuildConfig, error) {
	stream, err := source.StreamFile(configSource.Path)
	if err != nil {
		return atc.BuildConfig{}, err
	}

	defer stream.Close()

	var config atc.BuildConfig
	err = candiedyaml.NewDecoder(stream).Decode(&config)
	if err != nil {
		return atc.BuildConfig{}, err
	}

	return config, nil
}

type MergedConfigSource struct {
	A BuildConfigSource
	B BuildConfigSource
}

func (configSource MergedConfigSource) FetchConfig(source ArtifactSource) (atc.BuildConfig, error) {
	aConfig, err := configSource.A.FetchConfig(source)
	if err != nil {
		return atc.BuildConfig{}, err
	}

	bConfig, err := configSource.B.FetchConfig(source)
	if err != nil {
		return atc.BuildConfig{}, err
	}

	return aConfig.Merge(bConfig), nil
}
