package atc

type Plan struct {
	Compose     *ComposePlan     `json:"compose,omitempty"`
	Aggregate   *AggregatePlan   `json:"aggregate,omitempty"`
	Get         *GetPlan         `json:"get,omitempty"`
	Put         *PutPlan         `json:"put,omitempty"`
	Execute     *ExecutePlan     `json:"execute,omitempty"`
	Conditional *ConditionalPlan `json:"conditional,omitempty"`
}

type ComposePlan struct {
	A Plan `json:"a"`
	B Plan `json:"b"`
}

type AggregatePlan map[string]Plan

type GetPlan struct {
	Type     string  `json:"type"`
	Name     string  `json:"name,omitempty"`
	Resource string  `json:"resource"`
	Source   Source  `json:"source"`
	Params   Params  `json:"params,omitempty"`
	Version  Version `json:"version,omitempty"`
}

type PutPlan struct {
	Type     string `json:"type"`
	Resource string `json:"resource"`
	Source   Source `json:"source"`
	Params   Params `json:"params,omitempty"`
}

type ExecutePlan struct {
	Privileged bool `json:"privileged"`

	ConfigPath string       `json:"config_path,omitempty"`
	Config     *BuildConfig `json:"config,omitempty"`
}

type ConditionalPlan struct {
	Conditions Conditions `json:"conditions"`
	Plan       Plan       `json:"plan"`
}
