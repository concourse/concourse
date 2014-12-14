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
)

type Build struct {
	ID     int
	Name   string
	Status Status

	JobName string

	Engine         string
	EngineMetadata string

	StartTime time.Time
	EndTime   time.Time
}

func (b Build) OneOff() bool {
	return b.JobName == ""
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

type VersionedResource struct {
	Resource string
	Type     string
	Source   Source
	Version  Version
	Metadata []MetadataField
}

type Source map[string]interface{}

type Version map[string]interface{}

type MetadataField struct {
	Name  string
	Value string
}
