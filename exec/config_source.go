package exec

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/concourse/atc"
	"gopkg.in/yaml.v2"
)

//go:generate counterfeiter . TaskConfigSource
type TaskConfigSource interface {
	FetchConfig(*SourceRepository) (atc.TaskConfig, error)
}

type StaticConfigSource struct {
	Config atc.TaskConfig
}

func (configSource StaticConfigSource) FetchConfig(*SourceRepository) (atc.TaskConfig, error) {
	return configSource.Config, nil
}

type FileConfigSource struct {
	Path string
}

type UnknownArtifactSourceError struct {
	SourceName SourceName
}

func (err UnknownArtifactSourceError) Error() string {
	return fmt.Sprintf("unknown artifact source: %s", err.SourceName)
}

type UnspecifiedArtifactSourceError struct {
	Path string
}

func (err UnspecifiedArtifactSourceError) Error() string {
	return fmt.Sprintf("config path '%s' does not specify where the file lives", err.Path)
}

func (configSource FileConfigSource) FetchConfig(repo *SourceRepository) (atc.TaskConfig, error) {
	segs := strings.SplitN(configSource.Path, "/", 2)
	if len(segs) != 2 {
		return atc.TaskConfig{}, UnspecifiedArtifactSourceError{configSource.Path}
	}

	sourceName := SourceName(segs[0])
	filePath := segs[1]

	source, found := repo.SourceFor(sourceName)
	if !found {
		return atc.TaskConfig{}, UnknownArtifactSourceError{sourceName}
	}

	stream, err := source.StreamFile(filePath)
	if err != nil {
		return atc.TaskConfig{}, err
	}

	defer stream.Close()

	streamedFile, err := ioutil.ReadAll(stream)
	if err != nil {
		return atc.TaskConfig{}, err
	}

	var config atc.TaskConfig
	if err := yaml.Unmarshal(streamedFile, &config); err != nil {
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

func (configSource MergedConfigSource) FetchConfig(source *SourceRepository) (atc.TaskConfig, error) {
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
