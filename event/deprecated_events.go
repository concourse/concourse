package event

import "github.com/concourse/atc"

// move events here as they cease to be emitted by new code.
//
// denormalize the any external consts (e.g. type) so these are self-contained.

type InputV10 struct {
	Input LegacyTurbineInput `json:"input"`
}

type LegacyTurbineInput struct {
	Name     string                 `json:"name"`
	Resource string                 `json:"resource"`
	Type     string                 `json:"type"`
	Version  map[string]interface{} `json:"version,omitempty"`
	Source   map[string]interface{} `json:"source,omitempty"`
	Params   map[string]interface{} `json:"params,omitempty"`
	Metadata []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"metadata,omitempty"`
	ConfigPath string `json:"config_path"`
}

func (InputV10) EventType() atc.EventType  { return "input" }
func (InputV10) Version() atc.EventVersion { return "1.0" }
func (e InputV10) Censored() atc.Event {
	e.Input.Source = nil
	e.Input.Params = nil
	return e
}

type OutputV10 struct {
	Output LegacyTurbineOutput `json:"output"`
}

type LegacyTurbineOutput struct {
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	On       []string               `json:"on,omitempty"`
	Version  map[string]interface{} `json:"version,omitempty"`
	Source   map[string]interface{} `json:"source,omitempty"`
	Params   map[string]interface{} `json:"params,omitempty"`
	Metadata []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"metadata,omitempty"`
}

func (OutputV10) EventType() atc.EventType  { return "output" }
func (OutputV10) Version() atc.EventVersion { return "1.0" }
func (e OutputV10) Censored() atc.Event {
	e.Output.Source = nil
	e.Output.Params = nil
	return e
}

type LogV10 struct {
	Origin  OriginV10 `json:"origin"`
	Payload string    `json:"payload"`
}

func (LogV10) EventType() atc.EventType  { return "log" }
func (LogV10) Version() atc.EventVersion { return "1.0" }
func (e LogV10) Censored() atc.Event     { return e }

type OriginV10 struct {
	Type OriginV10Type `json:"type"`
	Name string        `json:"name"`
}

type OriginV10Type string

const (
	OriginV10TypeInvalid OriginType = ""
	OriginV10TypeInput   OriginType = "input"
	OriginV10TypeOutput  OriginType = "output"
	OriginV10TypeRun     OriginType = "run"
)

type FinishV10 struct {
	Time       int64 `json:"time"`
	ExitStatus int   `json:"exit_status"`
}

func (FinishV10) EventType() atc.EventType  { return "finish" }
func (FinishV10) Version() atc.EventVersion { return "1.0" }
func (e FinishV10) Censored() atc.Event     { return e }

type StartV10 struct {
	Time int64 `json:"time"`
}

func (StartV10) EventType() atc.EventType  { return "start" }
func (StartV10) Version() atc.EventVersion { return "1.0" }
func (e StartV10) Censored() atc.Event     { return e }

type InitializeV10 struct {
	TaskConfig atc.TaskConfig `json:"config"`
}

func (InitializeV10) EventType() atc.EventType  { return "initialize" }
func (InitializeV10) Version() atc.EventVersion { return "1.0" }
func (e InitializeV10) Censored() atc.Event {
	e.TaskConfig.Params = nil
	return e
}

type InputV20 struct {
	Plan            atc.InputPlan       `json:"plan"`
	FetchedVersion  atc.Version         `json:"version"`
	FetchedMetadata []atc.MetadataField `json:"metadata,omitempty"`
}

func (InputV20) EventType() atc.EventType  { return "input" }
func (InputV20) Version() atc.EventVersion { return "2.0" }
func (e InputV20) Censored() atc.Event {
	e.Plan.Source = nil
	e.Plan.Params = nil
	return e
}

type OutputV20 struct {
	Plan            atc.OutputPlan      `json:"plan"`
	CreatedVersion  atc.Version         `json:"version"`
	CreatedMetadata []atc.MetadataField `json:"metadata,omitempty"`
}

func (OutputV20) EventType() atc.EventType  { return "output" }
func (OutputV20) Version() atc.EventVersion { return "2.0" }
func (e OutputV20) Censored() atc.Event {
	e.Plan.Source = nil
	e.Plan.Params = nil
	return e
}
