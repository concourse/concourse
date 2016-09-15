package db

import (
	"time"

	"github.com/concourse/atc"
)

type Resource struct {
	Name string
}

type SavedResource struct {
	ID           int
	CheckError   error
	Paused       bool
	PipelineName string
	Config       atc.ResourceConfig
	Resource
}

type SavedResourceType struct {
	ID      int
	Name    string
	Type    string
	Version Version
	Config  atc.ResourceType
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

type SavedVersionedResource struct {
	ID int

	Enabled bool

	ModifiedTime time.Time

	VersionedResource

	CheckOrder int
}

type SavedVersionedResources []SavedVersionedResource

type Version map[string]string

type MetadataField struct {
	Name  string
	Value string
}
