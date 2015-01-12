package event

import "github.com/concourse/atc"

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

func (InputV10) EventType() atc.EventType  { return EventTypeInput }
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

func (OutputV10) EventType() atc.EventType  { return EventTypeOutput }
func (OutputV10) Version() atc.EventVersion { return "1.0" }
func (e OutputV10) Censored() atc.Event {
	e.Output.Source = nil
	e.Output.Params = nil
	return e
}
