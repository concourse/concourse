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

type Event struct {
	EventType     string
	ExitStatus    int
	VersionResult VersionResult
}

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

type Runner interface {
	RunScript(
		context.Context,
		string,
		[]string,
		[]byte,
		interface{},
		io.Writer,
		bool,
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
