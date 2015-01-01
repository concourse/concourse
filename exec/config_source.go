package exec

import (
	"errors"

	"github.com/concourse/atc"
)

//go:generate counterfeiter . BuildConfigSource
type BuildConfigSource interface {
	FetchConfig(ArtifactSource) (atc.BuildConfig, error)
}

type DirectConfigSource struct {
	Config atc.BuildConfig
}

func (source DirectConfigSource) FetchConfig(ArtifactSource) (atc.BuildConfig, error) {
	return source.Config, nil
}

type MergedConfigSource struct {
	A BuildConfigSource
	B BuildConfigSource
}

func (source MergedConfigSource) FetchConfig(ArtifactSource) (atc.BuildConfig, error) {
	return atc.BuildConfig{}, errors.New("not implemented")
}

type FileConfigSource struct {
	Path string
}

func (source FileConfigSource) FetchConfig(ArtifactSource) (atc.BuildConfig, error) {
	return atc.BuildConfig{}, errors.New("not implemented")
}
