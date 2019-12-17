package runtime

import (
	"context"
	"fmt"
	"io"

	"github.com/concourse/concourse/atc"
)

const (
	InitializingEvent          = "Initializing"
	StartingEvent              = "Starting"
	FinishedEvent              = "Finished"
	ResourceResultPropertyName = "concourse:resource-result"
	ResourceProcessID          = "resource"
)

type VersionResult struct {
	Version  atc.Version         `json:"version"`
	Metadata []atc.MetadataField `json:"metadata,omitempty"`
}

type PutRequest struct {
	Source atc.Source `json:"source"`
	Params atc.Params `json:"params,omitempty"`
}

type GetRequest struct {
	Source atc.Source `json:"source"`
	Params atc.Params `json:"params,omitempty"`
}

//go:generate counterfeiter . Artifact
type Artifact interface {
	ID() string
}

type CacheArtifact struct {
	TeamID   int
	JobID    int
	StepName string
	Path     string
}

func (art CacheArtifact) ID() string {
	return fmt.Sprintf("%d, %d, %s, %s", art.TeamID, art.JobID, art.StepName, art.Path)
}

type GetArtifact struct {
	VolumeHandle string
}

func (art GetArtifact) ID() string {
	return art.VolumeHandle
}

type TaskArtifact struct {
	VolumeHandle string
}

func (art TaskArtifact) ID() string {
	return art.VolumeHandle
}

// TODO (runtime/#4910): consider a different name as this is close to "Runnable" in atc/engine/engine
//go:generate counterfeiter . Runner
type Runner interface {
	RunScript(
		ctx context.Context,
		path string,
		args []string,
		input []byte,
		output interface{},
		logDest io.Writer,
		recoverable bool,
	) error
}

type ProcessSpec struct {
	Path         string
	Args         []string
	Dir          string
	User         string
	StdoutWriter io.Writer
	StderrWriter io.Writer
}
