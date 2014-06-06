package builds

import "github.com/winston-ci/winston/config"

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
	ID int

	AbortURL string

	Status Status

	Inputs []Input
}

type Input struct {
	Name     string
	Source   config.Source
	Version  Version
	Metadata []MetadataField
}

type Version map[string]interface{}

type MetadataField struct {
	Name  string
	Value string
}

type ByID []Build

func (bs ByID) Len() int           { return len(bs) }
func (bs ByID) Less(i, j int) bool { return bs[i].ID < bs[j].ID }
func (bs ByID) Swap(i, j int)      { bs[i], bs[j] = bs[j], bs[i] }
