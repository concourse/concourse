package runtime

import (
	"fmt"
	"io"

	"github.com/concourse/concourse/atc"
)

const (
	InitializingEvent = "Initializing"
	StartingEvent     = "Starting"
	FinishedEvent     = "Finished"
)

type Event struct {
	EventType     string
	ExitStatus    int
	VersionResult VersionResult
}

type IOConfig struct {
	Stdout io.Writer
	Stderr io.Writer
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

type TaskCacheArtifact struct {
	TeamID   int
	JobID    int
	StepName string
	Path     string
}

func (art TaskCacheArtifact) ID() string {
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

func (art *TaskArtifact) ID() string {
	return art.VolumeHandle
}

//type Runnable interface {
//	Destroy() error
//
//	VolumeMounts() []VolumeMount
//
//	WorkerName() string
//
//	MarkAsHijacked() error
//}
