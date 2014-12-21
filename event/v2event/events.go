package v2event

import "github.com/concourse/atc"

type Input struct {
	Plan            atc.InputPlan       `json:"plan"`
	FetchedVersion  atc.Version         `json:"version"`
	FetchedMetadata []atc.MetadataField `json:"metadata"`
}

func (Input) EventType() atc.EventType  { return EventTypeInput }
func (Input) Version() atc.EventVersion { return "2.0" }
func (e Input) Censored() atc.Event {
	e.Plan.Source = nil
	e.Plan.Params = nil
	return e
}

type Output struct {
	Plan            atc.OutputPlan      `json:"plan"`
	CreatedVersion  atc.Version         `json:"version"`
	CreatedMetadata []atc.MetadataField `json:"metadata"`
}

func (Output) EventType() atc.EventType  { return EventTypeOutput }
func (Output) Version() atc.EventVersion { return "2.0" }
func (e Output) Censored() atc.Event {
	e.Plan.Source = nil
	e.Plan.Params = nil
	return e
}
