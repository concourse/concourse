package event

import (
	"github.com/concourse/atc"
	"github.com/concourse/turbine"
)

type Error struct {
	Message string `json:"message"`
	Origin  Origin `json:"origin,omitempty"`
}

func (Error) EventType() atc.EventType  { return EventTypeError }
func (Error) Version() atc.EventVersion { return "1.0" }
func (e Error) Censored() atc.Event     { return e }

type Finish struct {
	Time       int64 `json:"time"`
	ExitStatus int   `json:"exit_status"`
}

func (Finish) EventType() atc.EventType  { return EventTypeFinish }
func (Finish) Version() atc.EventVersion { return "1.0" }
func (e Finish) Censored() atc.Event     { return e }

type Initialize struct {
	BuildConfig atc.BuildConfig `json:"config"`
}

func (Initialize) EventType() atc.EventType  { return EventTypeInitialize }
func (Initialize) Version() atc.EventVersion { return "1.0" }
func (e Initialize) Censored() atc.Event {
	e.BuildConfig.Params = nil
	return e
}

type InputV10 struct {
	Input turbine.Input `json:"input"`
}

func (InputV10) EventType() atc.EventType  { return EventTypeInput }
func (InputV10) Version() atc.EventVersion { return "1.0" }
func (e InputV10) Censored() atc.Event {
	e.Input.Source = nil
	e.Input.Params = nil
	return e
}

type Log struct {
	Origin  Origin `json:"origin"`
	Payload string `json:"payload"`
}

func (Log) EventType() atc.EventType  { return EventTypeLog }
func (Log) Version() atc.EventVersion { return "1.0" }
func (e Log) Censored() atc.Event     { return e }

type OutputV10 struct {
	Output turbine.Output `json:"output"`
}

func (OutputV10) EventType() atc.EventType  { return EventTypeOutput }
func (OutputV10) Version() atc.EventVersion { return "1.0" }
func (e OutputV10) Censored() atc.Event {
	e.Output.Source = nil
	e.Output.Params = nil
	return e
}

type Start struct {
	Time int64 `json:"time"`
}

func (Start) EventType() atc.EventType  { return EventTypeStart }
func (Start) Version() atc.EventVersion { return "1.0" }
func (e Start) Censored() atc.Event     { return e }

type Status struct {
	Status atc.BuildStatus `json:"status"`
	Time   int64           `json:"time"`
}

func (Status) EventType() atc.EventType  { return EventTypeStatus }
func (Status) Version() atc.EventVersion { return "1.0" }
func (e Status) Censored() atc.Event     { return e }

type Origin struct {
	Type OriginType `json:"type"`
	Name string     `json:"name"`
}

type OriginType string

const (
	OriginTypeInvalid OriginType = ""
	OriginTypeInput   OriginType = "input"
	OriginTypeOutput  OriginType = "output"
	OriginTypeRun     OriginType = "run"
)

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
