package db

import "github.com/concourse/atc"

type Pipeline struct {
	Name    string
	Config  atc.Config
	Version ConfigVersion
}

type SavedPipeline struct {
	ID int

	Pipeline
}
