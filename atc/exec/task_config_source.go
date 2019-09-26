package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"strconv"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/vars"
	"sigs.k8s.io/yaml"
)

//go:generate counterfeiter . TaskConfigSource

// TaskConfigSource is used to determine a Task step's TaskConfig.
type TaskConfigSource interface {
	// FetchConfig returns the TaskConfig, and may have to a task config file out
	// of the artifact.Repository.
	FetchConfig(context.Context, lager.Logger, *build.Repository) (atc.TaskConfig, error)
	Warnings() []string
}

// StaticConfigSource represents a statically configured TaskConfig.
type StaticConfigSource struct {
	Config *atc.TaskConfig
}

// FetchConfig returns the configuration.
func (configSource StaticConfigSource) FetchConfig(context.Context, lager.Logger, *build.Repository) (atc.TaskConfig, error) {
	taskConfig := atc.TaskConfig{}
	if configSource.Config != nil {
		taskConfig = *configSource.Config
	}
	return taskConfig, nil
}

func (configSource StaticConfigSource) Warnings() []string {
	return []string{}
}

// FileConfigSource represents a dynamically configured TaskConfig, which will
// be fetched from a specified file in the artifact.Repository.
type FileConfigSource struct {
	ConfigPath string
	Client     worker.Client
}

// FetchConfig reads the specified file from the artifact.Repository and loads the
// TaskConfig contained therein (expecting it to be YAML format).
//
// The path must be in the format SOURCE_NAME/FILE/PATH.yml. The SOURCE_NAME
// will be used to determine the ArtifactSource in the artifact.Repository to
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
func (configSource FileConfigSource) FetchConfig(ctx context.Context, logger lager.Logger, repo *build.Repository) (atc.TaskConfig, error) {
	segs := strings.SplitN(configSource.ConfigPath, "/", 2)
	if len(segs) != 2 {
		return atc.TaskConfig{}, UnspecifiedArtifactSourceError{configSource.ConfigPath}
	}

	sourceName := build.ArtifactName(segs[0])
	filePath := segs[1]

	artifact, found := repo.ArtifactFor(sourceName)
	if !found {
		return atc.TaskConfig{}, UnknownArtifactSourceError{sourceName, configSource.ConfigPath}
	}
	fmt.Println("================================= OMG task config source FetchConfig")
	fmt.Printf("\n\n%#v\n\n", artifact)
	fmt.Println(filePath)
	fmt.Println("================================= OMG task config source FetchConfig2")
	stream, err := configSource.Client.StreamFileFromArtifact(ctx, logger, artifact, filePath)
	if err != nil {
		if err == baggageclaim.ErrFileNotFound {
			return atc.TaskConfig{}, fmt.Errorf("task config '%s/%s' not found", sourceName, filePath)
		}
		return atc.TaskConfig{}, err
	}

	defer stream.Close()

	byteConfig, err := ioutil.ReadAll(stream)
	if err != nil {
		return atc.TaskConfig{}, err
	}

	config, err := atc.NewTaskConfig(byteConfig)
	if err != nil {
		return atc.TaskConfig{}, fmt.Errorf("failed to create task config from bytes %s: %s", configSource.ConfigPath, err)
	}

	return config, nil
}

func (configSource FileConfigSource) Warnings() []string {
	return []string{}
}

// OverrideParamsConfigSource is used to override params in a config source
type OverrideParamsConfigSource struct {
	ConfigSource TaskConfigSource
	Params       atc.Params
	WarningList  []string
}

// FetchConfig overrides parameters, allowing the user to set params required by a task loaded
// from a file by providing them in static configuration.
func (configSource *OverrideParamsConfigSource) FetchConfig(ctx context.Context, logger lager.Logger, source *build.Repository) (atc.TaskConfig, error) {
	taskConfig, err := configSource.ConfigSource.FetchConfig(ctx, logger, source)
	if err != nil {
		return atc.TaskConfig{}, err
	}

	if taskConfig.Params == nil {
		taskConfig.Params = atc.TaskEnv{}
	}

	for key, val := range configSource.Params {
		if _, exists := taskConfig.Params[key]; !exists {
			configSource.WarningList = append(configSource.WarningList, fmt.Sprintf("%s was defined in pipeline but missing from task file", key))
		}

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

func (configSource OverrideParamsConfigSource) Warnings() []string {
	return configSource.WarningList
}

// InterpolateTemplateConfigSource represents a config source interpolated by template vars
type InterpolateTemplateConfigSource struct {
	ConfigSource TaskConfigSource
	Vars         []vars.Variables
}

// FetchConfig returns the interpolated configuration
func (configSource InterpolateTemplateConfigSource) FetchConfig(ctx context.Context, logger lager.Logger, source *build.Repository) (atc.TaskConfig, error) {
	taskConfig, err := configSource.ConfigSource.FetchConfig(ctx, logger, source)
	if err != nil {
		return atc.TaskConfig{}, err
	}

	byteConfig, err := yaml.Marshal(taskConfig)
	if err != nil {
		return atc.TaskConfig{}, fmt.Errorf("failed to marshal task config: %s", err)
	}

	// process task config using the provided variables
	byteConfig, err = vars.NewTemplateResolver(byteConfig, configSource.Vars).Resolve(true, true)
	if err != nil {
		return atc.TaskConfig{}, fmt.Errorf("failed to interpolate task config: %s", err)
	}

	taskConfig, err = atc.NewTaskConfig(byteConfig)
	if err != nil {
		return atc.TaskConfig{}, fmt.Errorf("failed to create task config from bytes: %s", err)
	}

	return taskConfig, nil
}

func (configSource InterpolateTemplateConfigSource) Warnings() []string {
	return []string{}
}

// ValidatingConfigSource delegates to another ConfigSource, and validates its
// task config.
type ValidatingConfigSource struct {
	ConfigSource TaskConfigSource
}

// FetchConfig fetches the config using the underlying ConfigSource, and checks
// that it's valid.
func (configSource ValidatingConfigSource) FetchConfig(ctx context.Context, logger lager.Logger, source *build.Repository) (atc.TaskConfig, error) {
	config, err := configSource.ConfigSource.FetchConfig(ctx, logger, source)
	if err != nil {
		return atc.TaskConfig{}, err
	}

	if err := config.Validate(); err != nil {
		return atc.TaskConfig{}, err
	}

	return config, nil
}

func (configSource ValidatingConfigSource) Warnings() []string {
	return configSource.ConfigSource.Warnings()
}

// UnknownArtifactSourceError is returned when the artifact.ArtifactName specified by the
// path does not exist in the artifact.Repository.
type UnknownArtifactSourceError struct {
	SourceName build.ArtifactName
	ConfigPath string
}

// Error returns a human-friendly error message.
func (err UnknownArtifactSourceError) Error() string {
	return fmt.Sprintf("unknown artifact source: '%s' in task config file path '%s'", err.SourceName, err.ConfigPath)
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
