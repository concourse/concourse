package atc

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type Step struct {
	Config StepConfig
}

func (step *Step) UnmarshalJSON(data []byte) error {
	var deferred map[string]*json.RawMessage
	err := json.Unmarshal(data, &deferred)
	if err != nil {
		return err
	}

	var outerStep StepConfig
	for _, s := range stepPrecedence {
		_, found := deferred[s.Key]
		if !found {
			continue
		}

		step := s.New()

		err := step.ParseJSON(data)
		if err != nil {
			return err
		}

		if outerStep == nil {
			outerStep = step
		} else {
			outerStep.Wrap(step)
		}

		delete(deferred, s.Key)

		data, err = json.Marshal(deferred)
		if err != nil {
			return fmt.Errorf("re-marshal deferred parsing: %s", err)
		}
	}

	if outerStep == nil {
		return fmt.Errorf("no step configured")
	}

	step.Config = outerStep

	return nil
}

type StepConfig interface {
	ParseJSON([]byte) error

	Visit(StepVisitor) error
	Wrap(StepConfig)
}

type StepVisitor interface {
	VisitTask(*TaskStep) error
	VisitGet(*GetStep) error
	VisitPut(*PutStep) error
	VisitSetPipeline(*SetPipelineStep) error
	VisitLoadVar(*LoadVarStep) error
	VisitTry(*TryStep) error
	VisitDo(*DoStep) error
	VisitInParallel(*InParallelStep) error
	VisitAggregate(*AggregateStep) error
	VisitTimeout(*TimeoutStep) error
	VisitRetry(*RetryStep) error
	VisitOnSuccess(*OnSuccessStep) error
	VisitOnFailure(*OnFailureStep) error
	VisitOnAbort(*OnAbortStep) error
	VisitOnError(*OnErrorStep) error
	VisitEnsure(*EnsureStep) error
}

type stepFactory struct {
	Key string
	New func() StepConfig
}

var stepPrecedence = []stepFactory{
	{
		Key: "ensure",
		New: func() StepConfig { return &EnsureStep{} },
	},
	{
		Key: "on_success",
		New: func() StepConfig { return &OnSuccessStep{} },
	},
	{
		Key: "on_failure",
		New: func() StepConfig { return &OnFailureStep{} },
	},
	{
		Key: "on_abort",
		New: func() StepConfig { return &OnAbortStep{} },
	},
	{
		Key: "on_error",
		New: func() StepConfig { return &OnErrorStep{} },
	},
	{
		Key: "attempts",
		New: func() StepConfig { return &RetryStep{} },
	},
	{
		Key: "timeout",
		New: func() StepConfig { return &TimeoutStep{} },
	},
	{
		Key: "task",
		New: func() StepConfig { return &TaskStep{} },
	},
	{
		Key: "put",
		New: func() StepConfig { return &PutStep{} },
	},
	{
		Key: "get",
		New: func() StepConfig { return &GetStep{} },
	},
	{
		Key: "set_pipeline",
		New: func() StepConfig { return &SetPipelineStep{} },
	},
	{
		Key: "load_var",
		New: func() StepConfig { return &LoadVarStep{} },
	},
	{
		Key: "try",
		New: func() StepConfig { return &TryStep{} },
	},
	{
		Key: "do",
		New: func() StepConfig { return &DoStep{} },
	},
	{
		Key: "in_parallel",
		New: func() StepConfig { return &InParallelStep{} },
	},
	{
		Key: "aggregate",
		New: func() StepConfig { return &AggregateStep{} },
	},
}

type GetStep struct {
	Name     string         `json:"get"`
	Resource string         `json:"resource,omitempty"`
	Version  *VersionConfig `json:"version,omitempty"`
	Params   Params         `json:"params,omitempty"`
	Passed   []string       `json:"passed,omitempty"`
	Trigger  bool           `json:"trigger,omitempty"`
	Tags     Tags           `json:"tags,omitempty"`
}

func (step *GetStep) ParseJSON(data []byte) error {
	return unmarshalStrict(data, step)
}

func (step *GetStep) Wrap(StepConfig) {}

func (step *GetStep) Visit(v StepVisitor) error {
	return v.VisitGet(step)
}

type PutStep struct {
	Name      string        `json:"put"`
	Resource  string        `json:"resource,omitempty"`
	Params    Params        `json:"params,omitempty"`
	Inputs    *InputsConfig `json:"inputs,omitempty"`
	Tags      Tags          `json:"tags,omitempty"`
	GetParams Params        `json:"get_params,omitempty"`
}

func (step *PutStep) ParseJSON(data []byte) error {
	return unmarshalStrict(data, step)
}

func (step *PutStep) Wrap(StepConfig) {}

func (step *PutStep) Visit(v StepVisitor) error {
	return v.VisitPut(step)
}

type TaskStep struct {
	Name              string            `json:"task"`
	Privileged        bool              `json:"privileged,omitempty"`
	ConfigPath        string            `json:"file,omitempty"`
	Config            *TaskConfig       `json:"config,omitempty"`
	Params            Params            `json:"params,omitempty"`
	Vars              Params            `json:"vars,omitempty"`
	Tags              Tags              `json:"tags,omitempty"`
	InputMapping      map[string]string `json:"input_mapping,omitempty"`
	OutputMapping     map[string]string `json:"output_mapping,omitempty"`
	ImageArtifactName string            `json:"image,omitempty"`
}

func (step *TaskStep) ParseJSON(data []byte) error {
	return unmarshalStrict(data, step)
}

func (step *TaskStep) Wrap(StepConfig) {}

func (step *TaskStep) Visit(v StepVisitor) error {
	return v.VisitTask(step)
}

type SetPipelineStep struct {
	Name     string   `json:"set_pipeline"`
	File     string   `json:"file,omitempty"`
	Team     string   `json:"team,omitempty"`
	Vars     Params   `json:"vars,omitempty"`
	VarFiles []string `json:"var_files,omitempty"`
}

func (step *SetPipelineStep) ParseJSON(data []byte) error {
	return unmarshalStrict(data, step)
}

func (step *SetPipelineStep) Wrap(StepConfig) {}

func (step *SetPipelineStep) Visit(v StepVisitor) error {
	return v.VisitSetPipeline(step)
}

type LoadVarStep struct {
	Name   string `json:"load_var"`
	File   string `json:"file,omitempty"`
	Format string `json:"format,omitempty"`
	Reveal bool   `json:"reveal,omitempty"`
}

func (step *LoadVarStep) ParseJSON(data []byte) error {
	return unmarshalStrict(data, step)
}

func (step *LoadVarStep) Wrap(StepConfig) {}

func (step *LoadVarStep) Visit(v StepVisitor) error {
	return v.VisitLoadVar(step)
}

type TryStep struct {
	Step Step `json:"try"`
}

func (step *TryStep) ParseJSON(data []byte) error {
	return unmarshalStrict(data, step)
}

func (step *TryStep) Wrap(StepConfig) {}

func (step *TryStep) Visit(v StepVisitor) error {
	return v.VisitTry(step)
}

type DoStep struct {
	Steps []Step `json:"do"`
}

func (step *DoStep) ParseJSON(data []byte) error {
	return unmarshalStrict(data, step)
}

func (step *DoStep) Wrap(StepConfig) {}

func (step *DoStep) Visit(v StepVisitor) error {
	return v.VisitDo(step)
}

type AggregateStep struct {
	Steps []Step `json:"aggregate"`
}

func (step *AggregateStep) ParseJSON(data []byte) error {
	return unmarshalStrict(data, step)
}

func (step *AggregateStep) Wrap(StepConfig) {}

func (step *AggregateStep) Visit(v StepVisitor) error {
	return v.VisitAggregate(step)
}

type InParallelStep struct {
	Config InParallelConfig2 `json:"in_parallel"`
}

func (step *InParallelStep) ParseJSON(data []byte) error {
	return unmarshalStrict(data, step)
}

func (step *InParallelStep) Wrap(StepConfig) {}

func (step *InParallelStep) Visit(v StepVisitor) error {
	return v.VisitInParallel(step)
}

type InParallelConfig2 struct {
	Steps    []Step `json:"steps,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	FailFast bool   `json:"fail_fast,omitempty"`
}

func (c *InParallelConfig2) UnmarshalJSON(payload []byte) error {
	var data interface{}
	err := json.Unmarshal(payload, &data)
	if err != nil {
		return err
	}

	switch actual := data.(type) {
	case []interface{}:
		if err := json.Unmarshal(payload, &c.Steps); err != nil {
			return fmt.Errorf("failed to unmarshal parallel steps: %s", err)
		}
	case map[string]interface{}:
		// Used to avoid infinite recursion when unmarshalling this variant.
		type target InParallelConfig2

		var t target
		if err := json.Unmarshal(payload, &t); err != nil {
			return fmt.Errorf("failed to unmarshal parallel config: %s", err)
		}

		c.Steps, c.Limit, c.FailFast = t.Steps, t.Limit, t.FailFast
	default:
		return fmt.Errorf("wrong type for parallel config: %v", actual)
	}

	return nil
}

type RetryStep struct {
	Attempts int `json:"attempts"`

	Step StepConfig
}

func (step *RetryStep) ParseJSON(data []byte) error {
	return json.Unmarshal(data, step)
}

func (step *RetryStep) Wrap(sub StepConfig) {
	if step.Step != nil {
		step.Step.Wrap(sub)
	} else {
		step.Step = sub
	}
}

func (step *RetryStep) Visit(v StepVisitor) error {
	return v.VisitRetry(step)
}

type TimeoutStep struct {
	Duration string `json:"timeout"`

	Step StepConfig
}

func (step *TimeoutStep) ParseJSON(data []byte) error {
	// var s struct {
	// 	Duration string `json:"timeout"`
	// }
	// err := json.Unmarshal(data, &s)
	// if err != nil {
	// 	return err
	// }

	// dur, err := time.ParseDuration(s.Duration)
	// if err != nil {
	// 	return err
	// }

	// step.Duration = dur

	return json.Unmarshal(data, &step)
}

func (step *TimeoutStep) Wrap(sub StepConfig) {
	if step.Step != nil {
		step.Step.Wrap(sub)
	} else {
		step.Step = sub
	}
}

func (step *TimeoutStep) Visit(v StepVisitor) error {
	return v.VisitTimeout(step)
}

type OnSuccessStep struct {
	Step StepConfig
	Hook Step `json:"on_success"`
}

func (step *OnSuccessStep) ParseJSON(data []byte) error {
	return json.Unmarshal(data, step)
}

func (step *OnSuccessStep) Wrap(sub StepConfig) {
	if step.Step != nil {
		step.Step.Wrap(sub)
	} else {
		step.Step = sub
	}
}

func (step *OnSuccessStep) Visit(v StepVisitor) error {
	return v.VisitOnSuccess(step)
}

type OnFailureStep struct {
	Step StepConfig
	Hook Step `json:"on_failure"`
}

func (step *OnFailureStep) ParseJSON(data []byte) error {
	return json.Unmarshal(data, step)
}

func (step *OnFailureStep) Wrap(sub StepConfig) {
	if step.Step != nil {
		step.Step.Wrap(sub)
	} else {
		step.Step = sub
	}
}

func (step *OnFailureStep) Visit(v StepVisitor) error {
	return v.VisitOnFailure(step)
}

type OnErrorStep struct {
	Step StepConfig
	Hook Step `json:"on_error"`
}

func (step *OnErrorStep) ParseJSON(data []byte) error {
	return json.Unmarshal(data, step)
}

func (step *OnErrorStep) Wrap(sub StepConfig) {
	if step.Step != nil {
		step.Step.Wrap(sub)
	} else {
		step.Step = sub
	}
}

func (step *OnErrorStep) Visit(v StepVisitor) error {
	return v.VisitOnError(step)
}

type OnAbortStep struct {
	Step StepConfig
	Hook Step `json:"on_abort"`
}

func (step *OnAbortStep) ParseJSON(data []byte) error {
	return json.Unmarshal(data, step)
}

func (step *OnAbortStep) Wrap(sub StepConfig) {
	if step.Step != nil {
		step.Step.Wrap(sub)
	} else {
		step.Step = sub
	}
}

func (step *OnAbortStep) Visit(v StepVisitor) error {
	return v.VisitOnAbort(step)
}

type EnsureStep struct {
	Step StepConfig
	Hook Step `json:"ensure"`
}

func (step *EnsureStep) ParseJSON(data []byte) error {
	return json.Unmarshal(data, step)
}

func (step *EnsureStep) Wrap(sub StepConfig) {
	if step.Step != nil {
		step.Step.Wrap(sub)
	} else {
		step.Step = sub
	}
}

func (step *EnsureStep) Visit(v StepVisitor) error {
	return v.VisitEnsure(step)
}

func unmarshalStrict(data []byte, to interface{}) error {
	decoder := json.NewDecoder(bytes.NewBuffer(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(to)
}
