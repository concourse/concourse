package db

import "time"

type Status string

const (
	StatusPending   Status = "pending"
	StatusStarted   Status = "started"
	StatusAborted   Status = "aborted"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusErrored   Status = "errored"
	StatusPaused    Status = "paused"
)

type Build struct {
	ID        int
	Name      string
	Status    Status
	Scheduled bool

	JobID        int
	JobName      string
	PipelineName string

	Engine         string
	EngineMetadata string

	StartTime time.Time
	EndTime   time.Time
}

func (b Build) OneOff() bool {
	return b.JobName == ""
}

func (b Build) Abortable() bool {
	switch b.Status {
	case StatusPending, StatusStarted:
		return true
	default:
		return false
	}
}

type Resource struct {
	Name string
}

type SavedResource struct {
	ID           int
	CheckError   error
	Paused       bool
	PipelineName string
	Resource
}

func (r SavedResource) FailingToCheck() bool {
	return r.CheckError != nil
}

type VersionedResource struct {
	Resource     string
	Type         string
	Source       Source
	Version      Version
	Metadata     []MetadataField
	PipelineName string
}

type VersionedResources []VersionedResource

func (vrs VersionedResources) Lookup(name string) (VersionedResource, bool) {
	for _, vr := range vrs {
		if vr.Resource == name {
			return vr, true
		}
	}

	return VersionedResource{}, false
}

type SavedVersionedResource struct {
	ID int

	Enabled bool

	VersionedResource
}

type SavedVersionedResources []SavedVersionedResource

func (vrs SavedVersionedResources) Lookup(name string) (SavedVersionedResource, bool) {
	for _, vr := range vrs {
		if vr.Resource == name {
			return vr, true
		}
	}

	return SavedVersionedResource{}, false
}

type Source map[string]interface{}

type Version map[string]interface{}

type MetadataField struct {
	Name  string
	Value string
}
