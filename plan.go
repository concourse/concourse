package atc

type Plan struct {
	Compose      *ComposePlan      `json:"compose,omitempty"`
	Aggregate    *AggregatePlan    `json:"aggregate,omitempty"`
	Get          *GetPlan          `json:"get,omitempty"`
	Put          *PutPlan          `json:"put,omitempty"`
	Task         *TaskPlan         `json:"task,omitempty"`
	Conditional  *ConditionalPlan  `json:"conditional,omitempty"`
	Ensure       *EnsurePlan       `json:"ensure,omitempty"`
	OnSuccess    *OnSuccessPlan    `json:"on_success,omitempty"`
	OnFailure    *OnFailurePlan    `json:"on_failure,omitempty"`
	Try          *TryPlan          `json:"try,omitempty"`
	Location     *Location         `json:"location,omitempty"`
	DependentGet *DependentGetPlan `json:"dependent_get,omitempty"`
	Timeout      *TimeoutPlan      `json:"timeout,omitempty"`
}

type DependentGetPlan struct {
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	Resource string `json:"resource"`
	Pipeline string `json:"pipeline"`
	Params   Params `json:"params,omitempty"`
	Tags     Tags   `json:"tags,omitempty"`
	Source   Source `json:"source"`
	Timeout  string `json:"timeout,omitempty"`
}

type Location struct {
	ParentID      uint `json:"parent_id,omitempty"`
	ParallelGroup uint `json:"parallel_group,omitempty"`
	ID            uint `json:"id,omitempty"`
	Hook          string
}

type ComposePlan struct {
	A Plan `json:"a"`
	B Plan `json:"b"`
}

type OnFailurePlan struct {
	Step Plan `json: "step"`
	Next Plan `json: "on_failure"`
}
type EnsurePlan struct {
	Step Plan `json: "step"`
	Next Plan `json: "ensure"`
}
type OnSuccessPlan struct {
	Step Plan `json: "step"`
	Next Plan `json: "on_success"`
}

type TimeoutPlan struct {
	Step     Plan   `json: "step"`
	Duration string `json:"duration"`
}

type TryPlan struct {
	Step Plan `json: "step"`
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
	Tags     Tags    `json:"tags,omitempty"`
	Timeout  string  `json:"timeout,omitempty"`
}

type PutPlan struct {
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	Resource string `json:"resource"`
	Pipeline string `json:"pipeline"`
	Source   Source `json:"source"`
	Params   Params `json:"params,omitempty"`
	Tags     Tags   `json:"tags,omitempty"`
	Timeout  string `json:"timeout,omitempty"`
}

func (plan DependentGetPlan) GetPlan() GetPlan {
	return GetPlan{
		Type:     plan.Type,
		Name:     plan.Name,
		Resource: plan.Resource,
		Pipeline: plan.Pipeline,
		Source:   plan.Source,
		Tags:     plan.Tags,
		Timeout:  plan.Timeout,
		Params:   plan.Params,
	}
}

type TaskPlan struct {
	Name string `json:"name,omitempty"`

	Privileged bool   `json:"privileged"`
	Tags       Tags   `json:"tags,omitempty"`
	Timeout    string `json:"timeout,omitempty"`

	ConfigPath string      `json:"config_path,omitempty"`
	Config     *TaskConfig `json:"config,omitempty"`
}

type ConditionalPlan struct {
	Conditions Conditions `json:"conditions"`
	Plan       Plan       `json:"plan"`
}
