package exec

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"strconv"
	"strings"

	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
	"github.com/concourse/baggageclaim"
)

//go:generate counterfeiter . TaskConfigFetcher

// TaskConfigFetcher is used to determine a Task step's TaskConfig.
type TaskConfigFetcher interface {
	// FetchConfig returns the TaskConfig, and may have to a task config file out
	// of the worker.ArtifactRepository.
	FetchConfig(*worker.ArtifactRepository) (atc.TaskConfig, error)
	Warnings() []string
}

// StaticConfigFetcher represents a statically configured TaskConfig.
type StaticConfigFetcher struct {
	Plan atc.TaskPlan
}

// FetchConfig returns the configuration.
func (configFetcher StaticConfigFetcher) FetchConfig(*worker.ArtifactRepository) (atc.TaskConfig, error) {
	taskConfig := atc.TaskConfig{}

	if configFetcher.Plan.Config != nil {
		taskConfig = *configFetcher.Plan.Config
	}

	if configFetcher.Plan.Params == nil {
		return taskConfig, nil
	}

	if taskConfig.Params == nil {
		taskConfig.Params = map[string]string{}
	}

	for key, val := range configFetcher.Plan.Params {
		switch v := val.(type) {
		case string:
			taskConfig.Params[key] = v
		case float64:
			if math.Floor(v) == v {
				taskConfig.Params[key] = strconv.FormatInt(int64(v), 10)
			} else {
				taskConfig.Params[key] = strconv.FormatFloat(v, 'f', -1, 64)
			}
		default:
			bs, err := json.Marshal(val)
			if err != nil {
				return atc.TaskConfig{}, err
			}
			taskConfig.Params[key] = string(bs)
		}
	}

	return taskConfig, nil
}

func (configFetcher StaticConfigFetcher) Warnings() []string {
	return []string{}
}

// DeprecationConfigFetcher returns the Delegate TaskConfig and prints warnings to Stderr.
type DeprecationConfigFetcher struct {
	Delegate TaskConfigFetcher
	Stderr   io.Writer
}

// FetchConfig calls the Delegate's FetchConfig and prints warnings to Stderr
func (configFetcher DeprecationConfigFetcher) FetchConfig(repo *worker.ArtifactRepository) (atc.TaskConfig, error) {
	taskConfig, err := configFetcher.Delegate.FetchConfig(repo)
	if err != nil {
		return atc.TaskConfig{}, err
	}

	for _, warning := range configFetcher.Delegate.Warnings() {
		fmt.Fprintln(configFetcher.Stderr, warning)
	}

	return taskConfig, nil
}

func (configFetcher DeprecationConfigFetcher) Warnings() []string {
	return []string{}
}

// FileConfigFetcher represents a dynamically configured TaskConfig, which will
// be fetched from a specified file in the worker.ArtifactRepository.
type FileConfigFetcher struct {
	Path string
}

// FetchConfig reads the specified file from the worker.ArtifactRepository and loads the
// TaskConfig contained therein (expecting it to be YAML format).
//
// The path must be in the format SOURCE_NAME/FILE/PATH.yml. The SOURCE_NAME
// will be used to determine the ArtifactSource in the worker.ArtifactRepository to
// stream the file out of.
//
// If the source name is missing (i.e. if the path is just "foo.yml"),
// UnspecifiedArtifactSourceError is returned.
//
// If the specified source name cannot be found, UnknownArtifactSourceError is
// returned.
//
// If the task config file is not found, or is invalid YAML, or is an invalid
// task configuration, the respective errors will be bubbled up.
func (configFetcher FileConfigFetcher) FetchConfig(repo *worker.ArtifactRepository) (atc.TaskConfig, error) {
	segs := strings.SplitN(configFetcher.Path, "/", 2)
	if len(segs) != 2 {
		return atc.TaskConfig{}, UnspecifiedArtifactSourceError{configFetcher.Path}
	}

	sourceName := worker.ArtifactName(segs[0])
	filePath := segs[1]

	source, found := repo.SourceFor(sourceName)
	if !found {
		return atc.TaskConfig{}, UnknownArtifactSourceError{sourceName}
	}

	stream, err := source.StreamFile(filePath)
	if err != nil {
		if err == baggageclaim.ErrFileNotFound {
			return atc.TaskConfig{}, fmt.Errorf("task config '%s/%s' not found", sourceName, filePath)
		}
		return atc.TaskConfig{}, err
	}

	defer stream.Close()

	streamedFile, err := ioutil.ReadAll(stream)
	if err != nil {
		return atc.TaskConfig{}, err
	}

	config, err := atc.NewTaskConfig(streamedFile)
	if err != nil {
		return atc.TaskConfig{}, fmt.Errorf("failed to load %s: %s", configFetcher.Path, err)
	}

	return config, nil
}

func (configFetcher FileConfigFetcher) Warnings() []string {
	return []string{}
}

// MergedConfigFetcher is used to join two config sources together.
type MergedConfigFetcher struct {
	A TaskConfigFetcher
	B TaskConfigFetcher
}

// FetchConfig fetches both config sources, and merges the second config source
// into the first. This allows the user to set params required by a task loaded
// from a file by providing them in static configuration.
func (configFetcher MergedConfigFetcher) FetchConfig(source *worker.ArtifactRepository) (atc.TaskConfig, error) {
	aConfig, err := configFetcher.A.FetchConfig(source)
	if err != nil {
		return atc.TaskConfig{}, err
	}

	bConfig, err := configFetcher.B.FetchConfig(source)
	if err != nil {
		return atc.TaskConfig{}, err
	}

	return aConfig.Merge(bConfig), nil
}

func (configFetcher MergedConfigFetcher) Warnings() []string {
	warnings := []string{}
	warnings = append(warnings, configFetcher.A.Warnings()...)
	warnings = append(warnings, configFetcher.B.Warnings()...)

	return warnings
}

// ValidatingConfigFetcher delegates to another ConfigFetcher, and validates its
// task config.
type ValidatingConfigFetcher struct {
	ConfigFetcher TaskConfigFetcher
}

// FetchConfig fetches the config using the underlying ConfigFetcher, and checks
// that it's valid.
func (configFetcher ValidatingConfigFetcher) FetchConfig(source *worker.ArtifactRepository) (atc.TaskConfig, error) {
	config, err := configFetcher.ConfigFetcher.FetchConfig(source)
	if err != nil {
		return atc.TaskConfig{}, err
	}

	if err := config.Validate(); err != nil {
		return atc.TaskConfig{}, err
	}

	return config, nil
}

func (configFetcher ValidatingConfigFetcher) Warnings() []string {
	return configFetcher.ConfigFetcher.Warnings()
}

// UnknownArtifactSourceError is returned when the worker.ArtifactName specified by the
// path does not exist in the worker.ArtifactRepository.
type UnknownArtifactSourceError struct {
	SourceName worker.ArtifactName
}

// Error returns a human-friendly error message.
func (err UnknownArtifactSourceError) Error() string {
	return fmt.Sprintf("unknown artifact source: %s", err.SourceName)
}

// UnspecifiedArtifactSourceError is returned when the specified path is of a
// file in the toplevel directory, and so it does not indicate a SourceName.
type UnspecifiedArtifactSourceError struct {
	Path string
}

// Error returns a human-friendly error message.
func (err UnspecifiedArtifactSourceError) Error() string {
	return fmt.Sprintf("config path '%s' does not specify where the file lives", err.Path)
}
