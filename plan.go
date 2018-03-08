package atc

type Plan struct {
	ID       PlanID `json:"id"`
	Attempts []int  `json:"attempts,omitempty"`

	Aggregate *AggregatePlan `json:"aggregate,omitempty"`
	Do        *DoPlan        `json:"do,omitempty"`
	Get       *GetPlan       `json:"get,omitempty"`
	Put       *PutPlan       `json:"put,omitempty"`
	Task      *TaskPlan      `json:"task,omitempty"`
	OnAbort   *OnAbortPlan   `json:"on_abort,ommitempty"`
	Ensure    *EnsurePlan    `json:"ensure,omitempty"`
	OnSuccess *OnSuccessPlan `json:"on_success,omitempty"`
	OnFailure *OnFailurePlan `json:"on_failure,omitempty"`
	Try       *TryPlan       `json:"try,omitempty"`
	Timeout   *TimeoutPlan   `json:"timeout,omitempty"`
	Retry     *RetryPlan     `json:"retry,omitempty"`

	// deprecated, kept for backwards compatibility to be able to show old builds
	DependentGet *DependentGetPlan `json:"dependent_get,omitempty"`
}

type PlanID string

type OnAbortPlan struct {
	Step Plan `json:"step"`
	Next Plan `json:"on_abort"`
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
	Type        string   `json:"type"`
	Name        string   `json:"name,omitempty"`
	Resource    string   `json:"resource"`
	Source      Source   `json:"source"`
	Params      Params   `json:"params,omitempty"`
	Version     *Version `json:"version,omitempty"`
	VersionFrom *PlanID  `json:"version_from,omitempty"`
	Tags        Tags     `json:"tags,omitempty"`

	VersionedResourceTypes VersionedResourceTypes `json:"resource_types,omitempty"`
}

type PutPlan struct {
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	Resource string `json:"resource"`
	Source   Source `json:"source"`
	Params   Params `json:"params,omitempty"`
	Tags     Tags   `json:"tags,omitempty"`

	VersionedResourceTypes VersionedResourceTypes `json:"resource_types,omitempty"`
}

type TaskPlan struct {
	Name string `json:"name,omitempty"`

	Privileged bool `json:"privileged"`
	Tags       Tags `json:"tags,omitempty"`

	ConfigPath string      `json:"config_path,omitempty"`
	Config     *TaskConfig `json:"config,omitempty"`

	Params            Params            `json:"params,omitempty"`
	InputMapping      map[string]string `json:"input_mapping,omitempty"`
	OutputMapping     map[string]string `json:"output_mapping,omitempty"`
	ImageArtifactName string            `json:"image,omitempty"`

	VersionedResourceTypes VersionedResourceTypes `json:"resource_types,omitempty"`
}

type RetryPlan []Plan

type DependentGetPlan struct {
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	Resource string `json:"resource"`
}
