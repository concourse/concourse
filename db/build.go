package db

import (
	"time"

	"github.com/concourse/atc"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusStarted   Status = "started"
	StatusAborted   Status = "aborted"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusErrored   Status = "errored"
)

type Build struct {
	ID               int
	Name             string
	Status           Status
	Scheduled        bool
	InputsDetermined bool

	JobID        int
	JobName      string
	PipelineName string
	PipelineID   int

	Engine         string
	EngineMetadata string

	StartTime time.Time
	EndTime   time.Time
	ReapTime  time.Time
}

func (b Build) OneOff() bool {
	return b.JobName == ""
}

func (b Build) IsRunning() bool {
	switch b.Status {
	case StatusPending, StatusStarted:
		return true
	default:
		return false
	}
}

func (b Build) Abortable() bool {
	return b.IsRunning()
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

type DashboardResource struct {
	Resource       SavedResource
	ResourceConfig atc.ResourceConfig
}

type SavedResourceType struct {
	ID      int
	Name    string
	Type    string
	Version Version
}

func (r SavedResource) FailingToCheck() bool {
	return r.CheckError != nil
}

type VersionedResource struct {
	Resource   string
	Type       string
	Version    Version
	Metadata   []MetadataField
	PipelineID int
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

	ModifiedTime time.Time

	VersionedResource

	CheckOrder int
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

type Version map[string]string

type MetadataField struct {
	Name  string
	Value string
}
