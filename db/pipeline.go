package db

import "github.com/concourse/atc"

type Pipeline struct {
	Name    string
	Config  atc.Config
	Version ConfigVersion
}

type SavedPipeline struct {
	ID       int
	Paused   bool
	Public   bool
	TeamID   int
	TeamName string

	Pipeline
}
