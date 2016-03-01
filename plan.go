package atc

type Plan struct {
	ID       PlanID `json:"id"`
	Attempts []int  `json:"attempts,omitempty"`

	Aggregate    *AggregatePlan    `json:"aggregate,omitempty"`
	Do           *DoPlan           `json:"do,omitempty"`
	Get          *GetPlan          `json:"get,omitempty"`
	Put          *PutPlan          `json:"put,omitempty"`
	Task         *TaskPlan         `json:"task,omitempty"`
	Ensure       *EnsurePlan       `json:"ensure,omitempty"`
	OnSuccess    *OnSuccessPlan    `json:"on_success,omitempty"`
	OnFailure    *OnFailurePlan    `json:"on_failure,omitempty"`
	Try          *TryPlan          `json:"try,omitempty"`
	DependentGet *DependentGetPlan `json:"dependent_get,omitempty"`
	Timeout      *TimeoutPlan      `json:"timeout,omitempty"`
	Retry        *RetryPlan        `json:"retry,omitempty"`
}

type PlanID string

type DependentGetPlan struct {
	Type          string        `json:"type"`
	Name          string        `json:"name,omitempty"`
	Resource      string        `json:"resource"`
	ResourceTypes ResourceTypes `json:"resource_types,omitempty"`
	Pipeline      string        `json:"pipeline"`
	Params        Params        `json:"params,omitempty"`
	Tags          Tags          `json:"tags,omitempty"`
	Source        Source        `json:"source"`
}

func (plan DependentGetPlan) GetPlan() GetPlan {
	return GetPlan{
		Type:          plan.Type,
		Name:          plan.Name,
		Resource:      plan.Resource,
		ResourceTypes: plan.ResourceTypes,
		Pipeline:      plan.Pipeline,
		Source:        plan.Source,
		Tags:          plan.Tags,
		Params:        plan.Params,
	}
}

type OnFailurePlan struct {
	Step Plan `json:"step"`
	Next Plan `json:"on_failure"`
}

type EnsurePlan struct {
	Step Plan `json:"step"`
	Next Plan `json:"ensure"`
}

type OnSuccessPlan struct {
	Step Plan `json:"step"`
	Next Plan `json:"on_success"`
}

type TimeoutPlan struct {
	Step     Plan   `json:"step"`
	Duration string `json:"duration"`
}

type TryPlan struct {
	Step Plan `json:"step"`
}

type AggregatePlan []Plan

type DoPlan []Plan

type GetPlan struct {
	Type          string        `json:"type"`
	Name          string        `json:"name,omitempty"`
	Resource      string        `json:"resource"`
	ResourceTypes ResourceTypes `json:"resource_types,omitempty"`
	Pipeline      string        `json:"pipeline"`
	Source        Source        `json:"source"`
	Params        Params        `json:"params,omitempty"`
	Version       Version       `json:"version,omitempty"`
	Tags          Tags          `json:"tags,omitempty"`
}

type PutPlan struct {
	Type          string        `json:"type"`
	Name          string        `json:"name,omitempty"`
	Resource      string        `json:"resource"`
	ResourceTypes ResourceTypes `json:"resource_types,omitempty"`
	Pipeline      string        `json:"pipeline"`
	Source        Source        `json:"source"`
	Params        Params        `json:"params,omitempty"`
	Tags          Tags          `json:"tags,omitempty"`
}

type TaskPlan struct {
	Name string `json:"name,omitempty"`

	Privileged bool `json:"privileged"`
	Tags       Tags `json:"tags,omitempty"`

	ConfigPath string      `json:"config_path,omitempty"`
	Config     *TaskConfig `json:"config,omitempty"`

	Params Params `json:"params,omitempty"`

	Pipeline      string        `json:"pipeline"`
	ResourceTypes ResourceTypes `json:"resource_types,omitempty"`
}

type RetryPlan []Plan
