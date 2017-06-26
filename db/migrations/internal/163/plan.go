package migration_163

import "encoding/json"

type Plan struct {
	ID       PlanID `json:"id"`
	Attempts []int  `json:"attempts,omitempty"`

	Aggregate *AggregatePlan `json:"aggregate,omitempty"`
	Do        *DoPlan        `json:"do,omitempty"`
	Get       *GetPlan       `json:"get,omitempty"`
	Put       *PutPlan       `json:"put,omitempty"`
	Task      *TaskPlan      `json:"task,omitempty"`
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

	ConfigPath string          `json:"config_path,omitempty"`
	Config     *LoadTaskConfig `json:"config,omitempty"`

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

type Source map[string]interface{}

type Params map[string]interface{}

type Version map[string]string

type Tags []string

type VersionedResourceType struct {
	ResourceType

	Version Version `yaml:"version" json:"version" mapstructure:"version"`
}

type VersionedResourceTypes []VersionedResourceType

type ResourceType struct {
	Name       string `yaml:"name" json:"name" mapstructure:"name"`
	Type       string `yaml:"type" json:"type" mapstructure:"type"`
	Source     Source `yaml:"source" json:"source" mapstructure:"source"`
	Privileged bool   `yaml:"privileged,omitempty" json:"privileged" mapstructure:"privileged"`
	Tags       Tags   `yaml:"tags,omitempty" json:"tags" mapstructure:"tags"`
}

type LoadConfig struct {
	Path string `yaml:"load,omitempty" json:"load,omitempty" mapstructure:"load"`
}

type LoadTaskConfig struct {
	*LoadConfig
	*TaskConfig
}

type TaskConfig struct {
	// The platform the task must run on (e.g. linux, windows).
	Platform string `json:"platform,omitempty" yaml:"platform,omitempty" mapstructure:"platform"`

	// Optional string specifying an image to use for the build. Depending on the
	// platform, this may or may not be required (e.g. Windows/OS X vs. Linux).
	RootfsURI string `json:"rootfs_uri,omitempty" yaml:"rootfs_uri,omitempty" mapstructure:"rootfs_uri"`

	ImageResource *ImageResource `json:"image_resource,omitempty" yaml:"image_resource,omitempty" mapstructure:"image_resource"`

	// Parameters to pass to the task via environment variables.
	Params map[string]string `json:"params,omitempty" yaml:"params,omitempty" mapstructure:"params"`

	// Script to execute.
	Run TaskRunConfig `json:"run,omitempty" yaml:"run,omitempty" mapstructure:"run"`

	// The set of (logical, name-only) inputs required by the task.
	Inputs []TaskInputConfig `json:"inputs,omitempty" yaml:"inputs,omitempty" mapstructure:"inputs"`

	// The set of (logical, name-only) outputs provided by the task.
	Outputs []TaskOutputConfig `json:"outputs,omitempty" yaml:"outputs,omitempty" mapstructure:"outputs"`
}

type ImageResource struct {
	Type   string `yaml:"type" json:"type" mapstructure:"type"`
	Source Source `yaml:"source" json:"source" mapstructure:"source"`
}

type TaskRunConfig struct {
	Path string   `json:"path" yaml:"path"`
	Args []string `json:"args,omitempty" yaml:"args"`
	Dir  string   `json:"dir,omitempty" yaml:"dir"`

	// The user that the task will run as (defaults to whatever the docker image specifies)
	User string `json:"user,omitempty" yaml:"user,omitempty" mapstructure:"user"`
}

type TaskInputConfig struct {
	Name string `json:"name" yaml:"name"`
	Path string `json:"path,omitempty" yaml:"path"`
}

type TaskOutputConfig struct {
	Name string `json:"name" yaml:"name"`
	Path string `json:"path,omitempty" yaml:"path"`
}

type MetadataField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (plan Plan) Public() *json.RawMessage {
	var public struct {
		ID PlanID `json:"id"`

		Aggregate    *json.RawMessage `json:"aggregate,omitempty"`
		Do           *json.RawMessage `json:"do,omitempty"`
		Get          *json.RawMessage `json:"get,omitempty"`
		Put          *json.RawMessage `json:"put,omitempty"`
		Task         *json.RawMessage `json:"task,omitempty"`
		Ensure       *json.RawMessage `json:"ensure,omitempty"`
		OnSuccess    *json.RawMessage `json:"on_success,omitempty"`
		OnFailure    *json.RawMessage `json:"on_failure,omitempty"`
		Try          *json.RawMessage `json:"try,omitempty"`
		DependentGet *json.RawMessage `json:"dependent_get,omitempty"`
		Timeout      *json.RawMessage `json:"timeout,omitempty"`
		Retry        *json.RawMessage `json:"retry,omitempty"`
	}

	public.ID = plan.ID

	if plan.Aggregate != nil {
		public.Aggregate = plan.Aggregate.Public()
	}

	if plan.Do != nil {
		public.Do = plan.Do.Public()
	}

	if plan.Get != nil {
		public.Get = plan.Get.Public()
	}

	if plan.Put != nil {
		public.Put = plan.Put.Public()
	}

	if plan.Task != nil {
		public.Task = plan.Task.Public()
	}

	if plan.Ensure != nil {
		public.Ensure = plan.Ensure.Public()
	}

	if plan.OnSuccess != nil {
		public.OnSuccess = plan.OnSuccess.Public()
	}

	if plan.OnFailure != nil {
		public.OnFailure = plan.OnFailure.Public()
	}

	if plan.Try != nil {
		public.Try = plan.Try.Public()
	}

	if plan.Timeout != nil {
		public.Timeout = plan.Timeout.Public()
	}

	if plan.Retry != nil {
		public.Retry = plan.Retry.Public()
	}

	if plan.DependentGet != nil {
		public.DependentGet = plan.DependentGet.Public()
	}

	return enc(public)
}

func (plan AggregatePlan) Public() *json.RawMessage {
	public := make([]*json.RawMessage, len(plan))

	for i := 0; i < len(plan); i++ {
		public[i] = plan[i].Public()
	}

	return enc(public)
}

func (plan DoPlan) Public() *json.RawMessage {
	public := make([]*json.RawMessage, len(plan))

	for i := 0; i < len(plan); i++ {
		public[i] = plan[i].Public()
	}

	return enc(public)
}

func (plan EnsurePlan) Public() *json.RawMessage {
	return enc(struct {
		Step *json.RawMessage `json:"step"`
		Next *json.RawMessage `json:"ensure"`
	}{
		Step: plan.Step.Public(),
		Next: plan.Next.Public(),
	})
}

func (plan GetPlan) Public() *json.RawMessage {
	return enc(struct {
		Type     string   `json:"type"`
		Name     string   `json:"name,omitempty"`
		Resource string   `json:"resource"`
		Version  *Version `json:"version,omitempty"`
	}{
		Type:     plan.Type,
		Name:     plan.Name,
		Resource: plan.Resource,
		Version:  plan.Version,
	})
}

func (plan DependentGetPlan) Public() *json.RawMessage {
	return enc(struct {
		Type     string `json:"type"`
		Name     string `json:"name,omitempty"`
		Resource string `json:"resource"`
	}{
		Type:     plan.Type,
		Name:     plan.Name,
		Resource: plan.Resource,
	})
}

func (plan OnFailurePlan) Public() *json.RawMessage {
	return enc(struct {
		Step *json.RawMessage `json:"step"`
		Next *json.RawMessage `json:"on_failure"`
	}{
		Step: plan.Step.Public(),
		Next: plan.Next.Public(),
	})
}

func (plan OnSuccessPlan) Public() *json.RawMessage {
	return enc(struct {
		Step *json.RawMessage `json:"step"`
		Next *json.RawMessage `json:"on_success"`
	}{
		Step: plan.Step.Public(),
		Next: plan.Next.Public(),
	})
}

func (plan PutPlan) Public() *json.RawMessage {
	return enc(struct {
		Type     string `json:"type"`
		Name     string `json:"name,omitempty"`
		Resource string `json:"resource"`
	}{
		Type:     plan.Type,
		Name:     plan.Name,
		Resource: plan.Resource,
	})
}

func (plan TaskPlan) Public() *json.RawMessage {
	return enc(struct {
		Name       string `json:"name"`
		Privileged bool   `json:"privileged"`
	}{
		Name:       plan.Name,
		Privileged: plan.Privileged,
	})
}

func (plan TimeoutPlan) Public() *json.RawMessage {
	return enc(struct {
		Step     *json.RawMessage `json:"step"`
		Duration string           `json:"duration"`
	}{
		Step:     plan.Step.Public(),
		Duration: plan.Duration,
	})
}

func (plan TryPlan) Public() *json.RawMessage {
	return enc(struct {
		Step *json.RawMessage `json:"step"`
	}{
		Step: plan.Step.Public(),
	})
}

func (plan RetryPlan) Public() *json.RawMessage {
	public := make([]*json.RawMessage, len(plan))

	for i := 0; i < len(plan); i++ {
		public[i] = plan[i].Public()
	}

	return enc(public)
}

func enc(public interface{}) *json.RawMessage {
	enc, _ := json.Marshal(public)
	return (*json.RawMessage)(&enc)
}
