package atc

type Plan struct {
	Compose     *ComposePlan     `json:"compose,omitempty"`
	Aggregate   *AggregatePlan   `json:"aggregate,omitempty"`
	Get         *GetPlan         `json:"get,omitempty"`
	Put         *PutPlan         `json:"put,omitempty"`
	Task        *TaskPlan        `json:"task,omitempty"`
	Conditional *ConditionalPlan `json:"conditional,omitempty"`
	PutGet      *PutGetPlan      `json:"putget,omitempty"`
}

type ComposePlan struct {
	A Plan `json:"a"`
	B Plan `json:"b"`
}

type PutGetPlan struct {
	Head Plan `json:"put"`
	Rest Plan `json:"rest"`
}

type AggregatePlan []Plan

type GetPlan struct {
	Type     string  `json:"type"`
	Name     string  `json:"name,omitempty"`
	Resource string  `json:"resource"`
	Pipeline string  `json:"pipeline"`
	Source   Source  `json:"source"`
	Params   Params  `json:"params,omitempty"`
	Version  Version `json:"version,omitempty"`
}

type PutPlan struct {
	Type      string `json:"type"`
	Name      string `json:"name,omitempty"`
	Resource  string `json:"resource"`
	Pipeline  string `json:"pipeline"`
	Source    Source `json:"source"`
	Params    Params `json:"params,omitempty"`
	GetParams Params `json:"get_params,omitempty"`
}

func (plan PutPlan) GetPlan() GetPlan {
	return GetPlan{
		Type:     plan.Type,
		Name:     plan.Name,
		Resource: plan.Resource,
		Pipeline: plan.Pipeline,
		Source:   plan.Source,
		Params:   plan.GetParams,
	}
}

type TaskPlan struct {
	Name string `json:"name,omitempty"`

	Privileged bool `json:"privileged"`

	ConfigPath string      `json:"config_path,omitempty"`
	Config     *TaskConfig `json:"config,omitempty"`
}

type ConditionalPlan struct {
	Conditions Conditions `json:"conditions"`
	Plan       Plan       `json:"plan"`
}
