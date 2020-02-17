package atc

type Plan struct {
	ID       PlanID `json:"id"`
	Attempts []int  `json:"attempts,omitempty"`

	Aggregate   *AggregatePlan   `json:"aggregate,omitempty"`
	InParallel  *InParallelPlan  `json:"in_parallel,omitempty"`
	Do          *DoPlan          `json:"do,omitempty"`
	Get         *GetPlan         `json:"get,omitempty"`
	Put         *PutPlan         `json:"put,omitempty"`
	Check       *CheckPlan       `json:"check,omitempty"`
	Task        *TaskPlan        `json:"task,omitempty"`
	SetPipeline *SetPipelinePlan `json:"set_pipeline,omitempty"`
	LoadVar     *LoadVarPlan     `json:"load_var,omitempty"`
	OnAbort     *OnAbortPlan     `json:"on_abort,omitempty"`
	OnError     *OnErrorPlan     `json:"on_error,omitempty"`
	Ensure      *EnsurePlan      `json:"ensure,omitempty"`
	OnSuccess   *OnSuccessPlan   `json:"on_success,omitempty"`
	OnFailure   *OnFailurePlan   `json:"on_failure,omitempty"`
	Try         *TryPlan         `json:"try,omitempty"`
	Timeout     *TimeoutPlan     `json:"timeout,omitempty"`
	Interrupt   *InterruptPlan   `json:"interrupt_timeout,omitempty"`
	Retry       *RetryPlan       `json:"retry,omitempty"`

	// used for 'fly execute'
	ArtifactInput  *ArtifactInputPlan  `json:"artifact_input,omitempty"`
	ArtifactOutput *ArtifactOutputPlan `json:"artifact_output,omitempty"`

	// deprecated, kept for backwards compatibility to be able to show old builds
	DependentGet *DependentGetPlan `json:"dependent_get,omitempty"`
}

type PlanID string

type ArtifactInputPlan struct {
	ArtifactID int    `json:"artifact_id"`
	Name       string `json:"name"`
}

type ArtifactOutputPlan struct {
	Name string `json:"name"`
}

type OnAbortPlan struct {
	Step Plan `json:"step"`
	Next Plan `json:"on_abort"`
}

type OnErrorPlan struct {
	Step Plan `json:"step"`
	Next Plan `json:"on_error"`
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

type InterruptPlan struct {
	Step     Plan   `json:"step"`
	Duration string `json:"duration"`
}

type TryPlan struct {
	Step Plan `json:"step"`
}

type AggregatePlan []Plan

type InParallelPlan struct {
	Steps    []Plan `json:"steps"`
	Limit    int    `json:"limit,omitempty"`
	FailFast bool   `json:"fail_fast,omitempty"`
}

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
	Type     string        `json:"type"`
	Name     string        `json:"name,omitempty"`
	Resource string        `json:"resource"`
	Source   Source        `json:"source"`
	Params   Params        `json:"params,omitempty"`
	Tags     Tags          `json:"tags,omitempty"`
	Inputs   *InputsConfig `json:"inputs,omitempty"`

	VersionedResourceTypes VersionedResourceTypes `json:"resource_types,omitempty"`
}

type CheckPlan struct {
	Type        string  `json:"type"`
	Name        string  `json:"name,omitempty"`
	Source      Source  `json:"source"`
	Tags        Tags    `json:"tags,omitempty"`
	Timeout     string  `json:"timeout,omitempty"`
	FromVersion Version `json:"from_version,omitempty"`

	VersionedResourceTypes VersionedResourceTypes `json:"resource_types,omitempty"`
}

type TaskPlan struct {
	Name string `json:"name,omitempty"`

	Privileged bool `json:"privileged"`
	Tags       Tags `json:"tags,omitempty"`

	ConfigPath string      `json:"config_path,omitempty"`
	Config     *TaskConfig `json:"config,omitempty"`
	Vars       Params      `json:"vars,omitempty"`

	Params            Params            `json:"params,omitempty"`
	InputMapping      map[string]string `json:"input_mapping,omitempty"`
	OutputMapping     map[string]string `json:"output_mapping,omitempty"`
	ImageArtifactName string            `json:"image,omitempty"`

	VersionedResourceTypes VersionedResourceTypes `json:"resource_types,omitempty"`
}

type SetPipelinePlan struct {
	Name     string                 `json:"name"`
	File     string                 `json:"file"`
	Vars     map[string]interface{} `json:"vars,omitempty"`
	VarFiles []string               `json:"var_files,omitempty"`
}

type LoadVarPlan struct {
	Name   string `json:"name"`
	File   string `json:"file"`
	Format string `json:"format,omitempty"`
	Reveal bool   `json:"reveal,omitempty"`
}

type RetryPlan []Plan

type DependentGetPlan struct {
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	Resource string `json:"resource"`
}
