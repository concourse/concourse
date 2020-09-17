package atc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// Step is an "envelope" type, acting as a wrapper to handle the marshaling and
// unmarshaling of an underlying StepConfig.
type Step struct {
	Config        StepConfig
	UnknownFields map[string]*json.RawMessage
}

// ErrNoStepConfigured is returned when a step does not have any keys that
// indicate its step type.
var ErrNoStepConfigured = errors.New("no step configured")
var ErrNoCoreStepDeclared = errors.New("no core step type declared (e.g. get, put, task, etc.)")

// UnmarshalJSON unmarshals step configuration in multiple passes, determining
// precedence by the order of StepDetectors listed in the StepPrecedence
// variable.
//
// First, the step data is unmarshalled into a map[string]*json.RawMessage. Next,
// UnmarshalJSON loops over StepPrecedence to determine the type of step.
//
// For any StepDetector with a .Key field present in the map, .New is called to
// construct an empty StepConfig, and then json.Unmarshal is called on it to parse
// the data.
//
// For step modifiers like `timeout:` and `attempts:` they eventuallly wrap a
// core step type (e.g. get, put, task etc.). Core step types do not wrap other
// steps.
//
// When a core step type is encountered parsing stops and any remaining keys in
// rawStepConfig are considered invalid. This is how we stop someone from
// putting a `get` and `put` in the same step while still allowing valid step
// modifiers. This is also why step modifiers are listed first in
// StepPrecedence.
//
// If no StepDetectors match, no step is parsed, ErrNoStepConfigured is
// returned.
func (step *Step) UnmarshalJSON(data []byte) error {
	var rawStepConfig map[string]*json.RawMessage
	err := json.Unmarshal(data, &rawStepConfig)
	if err != nil {
		return err
	}

	var prevStep StepWrapper
	var coreStepDeclared bool
	for _, s := range StepPrecedence {
		_, found := rawStepConfig[s.Key]
		if !found {
			continue
		}

		curStep := s.New()

		err := json.Unmarshal(data, curStep)
		if err != nil {
			return MalformedStepError{
				StepType: s.Key,
				Err:      err,
			}
		}

		if step.Config == nil {
			step.Config = curStep
		}

		if prevStep != nil {
			prevStep.Wrap(curStep)
		}

		deleteKnownFields(rawStepConfig, curStep)

		if wrapper, isWrapper := curStep.(StepWrapper); isWrapper {
			prevStep = wrapper
		} else {
			coreStepDeclared = true
			break
		}

		data, err = json.Marshal(rawStepConfig)
		if err != nil {
			return fmt.Errorf("re-marshal rawStepConfig parsing: %w", err)
		}
	}

	if step.Config == nil {
		return ErrNoStepConfigured
	}

	if !coreStepDeclared {
		return ErrNoCoreStepDeclared
	}

	if len(rawStepConfig) != 0 {
		step.UnknownFields = rawStepConfig
	}

	return nil
}

// MarshalJSON marshals step configuration in multiple passes, looping and
// calling .Unwrap to marshal all nested steps into one big set of fields which
// is then marshalled and returned.
func (step Step) MarshalJSON() ([]byte, error) {
	fields := step.UnknownFields

	unwrapped := step.Config
	for unwrapped != nil {
		payload, err := json.Marshal(unwrapped)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(payload, &fields)
		if err != nil {
			return nil, err
		}

		if wrapper, isWrapper := unwrapped.(StepWrapper); isWrapper {
			unwrapped = wrapper.Unwrap()
		} else {
			break
		}
	}

	return json.Marshal(fields)
}

// See the note about json tags here: https://golang.org/pkg/encoding/json/#Marshal
func deleteKnownFields(rawStepConfig map[string]*json.RawMessage, step StepConfig) {
	stepType := reflect.TypeOf(step).Elem()
	for i := 0; i < stepType.NumField(); i++ {
		field := stepType.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		jsonTagParts := strings.Split(jsonTag, ",")
		if len(jsonTagParts) < 1 {
			continue
		}
		jsonKey := jsonTagParts[0]
		if jsonKey == "" {
			jsonKey = field.Name
		}
		delete(rawStepConfig, jsonKey)
	}
}

// StepConfig is implemented by all step types.
type StepConfig interface {
	// Visit must call StepVisitor with the appropriate method corresponding to
	// this step type.
	//
	// When a new step type is added, the StepVisitor interface must be extended.
	// This allows the compiler to help us track down all the places where steps
	// must be handled type-by-type.
	Visit(StepVisitor) error
}

// StepWrapper is an optional interface for step types that is implemented by
// steps that wrap/modify other steps (e.g. hooks like `on_success`, `timeout`, etc.)
type StepWrapper interface {
	// Wrap is called during (Step).UnmarshalJSON whenever an 'inner' step is
	// parsed.
	//
	// Modifier step types should implement this function by assigning the
	// passed in StepConfig to an internal field that has a `json:"-"` tag.
	Wrap(StepConfig)

	// Unwrap is called during (Step).MarshalJSON and must return the wrapped
	// StepConfig.
	Unwrap() StepConfig
}

// StepVisitor is an interface used to assist in finding all the places that
// need to be updated whenever a new step type is introduced.
//
// Each StepConfig must implement .Visit to call the appropriate method on the
// given StepVisitor.
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
	VisitAcross(*AcrossStep) error
	VisitTimeout(*TimeoutStep) error
	VisitRetry(*RetryStep) error
	VisitOnSuccess(*OnSuccessStep) error
	VisitOnFailure(*OnFailureStep) error
	VisitOnAbort(*OnAbortStep) error
	VisitOnError(*OnErrorStep) error
	VisitEnsure(*EnsureStep) error
}

// StepDetector is a simple structure used to detect whether a step type is
// configured.
type StepDetector struct {
	// Key is the field that, if present, indicates that the step is configured.
	Key string

	// If Key is present, New will be called to construct an empty StepConfig.
	New func() StepConfig
}

// StepPrecedence is a static list of all of the step types, listed in the
// order that they should be parsed. Broadly, modifiers are parsed first - with
// some important inter-modifier precedence - while core step types are parsed
// last.
var StepPrecedence = []StepDetector{
	{
		Key: "ensure",
		New: func() StepConfig { return &EnsureStep{} },
	},
	{
		Key: "on_error",
		New: func() StepConfig { return &OnErrorStep{} },
	},
	{
		Key: "on_abort",
		New: func() StepConfig { return &OnAbortStep{} },
	},
	{
		Key: "on_failure",
		New: func() StepConfig { return &OnFailureStep{} },
	},
	{
		Key: "on_success",
		New: func() StepConfig { return &OnSuccessStep{} },
	},
	{
		Key: "across",
		New: func() StepConfig { return &AcrossStep{} },
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

func (step *GetStep) ResourceName() string {
	if step.Resource != "" {
		return step.Resource
	}

	return step.Name
}

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

func (step *PutStep) ResourceName() string {
	if step.Resource != "" {
		return step.Resource
	}

	return step.Name
}

func (step *PutStep) Visit(v StepVisitor) error {
	return v.VisitPut(step)
}

type TaskStep struct {
	Name              string            `json:"task"`
	Privileged        bool              `json:"privileged,omitempty"`
	ConfigPath        string            `json:"file,omitempty"`
	Config            *TaskConfig       `json:"config,omitempty"`
	Params            TaskEnv           `json:"params,omitempty"`
	Vars              Params            `json:"vars,omitempty"`
	Tags              Tags              `json:"tags,omitempty"`
	InputMapping      map[string]string `json:"input_mapping,omitempty"`
	OutputMapping     map[string]string `json:"output_mapping,omitempty"`
	ImageArtifactName string            `json:"image,omitempty"`
}

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

func (step *SetPipelineStep) Visit(v StepVisitor) error {
	return v.VisitSetPipeline(step)
}

type LoadVarStep struct {
	Name   string `json:"load_var"`
	File   string `json:"file,omitempty"`
	Format string `json:"format,omitempty"`
	Reveal bool   `json:"reveal,omitempty"`
}

func (step *LoadVarStep) Visit(v StepVisitor) error {
	return v.VisitLoadVar(step)
}

type TryStep struct {
	Step Step `json:"try"`
}

func (step *TryStep) Visit(v StepVisitor) error {
	return v.VisitTry(step)
}

type DoStep struct {
	Steps []Step `json:"do"`
}

func (step *DoStep) Visit(v StepVisitor) error {
	return v.VisitDo(step)
}

type AggregateStep struct {
	Steps []Step `json:"aggregate"`
}

func (step *AggregateStep) Visit(v StepVisitor) error {
	return v.VisitAggregate(step)
}

type InParallelStep struct {
	Config InParallelConfig `json:"in_parallel"`
}

func (step *InParallelStep) Visit(v StepVisitor) error {
	return v.VisitInParallel(step)
}

type InParallelConfig struct {
	Steps    []Step `json:"steps,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	FailFast bool   `json:"fail_fast,omitempty"`
}

func (c *InParallelConfig) UnmarshalJSON(payload []byte) error {
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
		type target InParallelConfig

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

type AcrossVarConfig struct {
	Var         string             `json:"var"`
	Values      []interface{}      `json:"values,omitempty"`
	MaxInFlight *MaxInFlightConfig `json:"max_in_flight,omitempty"`
}

func (config *AcrossVarConfig) UnmarshalJSON(data []byte) error {
	// Used to avoid infinite recursion when unmarshalling.
	type target AcrossVarConfig

	var t target
	if err := unmarshalStrict(data, &t); err != nil {
		return err
	}

	*config = AcrossVarConfig(t)
	return nil
}

type AcrossStep struct {
	Step     StepConfig        `json:"-"`
	Vars     []AcrossVarConfig `json:"across"`
	FailFast bool              `json:"fail_fast,omitempty"`
}

func (step *AcrossStep) ParseJSON(data []byte) error {
	return json.Unmarshal(data, step)
}

func (step *AcrossStep) Visit(v StepVisitor) error {
	return v.VisitAcross(step)
}

func (step *AcrossStep) Wrap(sub StepConfig) {
	step.Step = sub
}

func (step *AcrossStep) Unwrap() StepConfig {
	return step.Step
}

type RetryStep struct {
	Step     StepConfig `json:"-"`
	Attempts int        `json:"attempts"`
}

func (step *RetryStep) Wrap(sub StepConfig) {
	step.Step = sub
}

func (step *RetryStep) Unwrap() StepConfig {
	return step.Step
}

func (step *RetryStep) Visit(v StepVisitor) error {
	return v.VisitRetry(step)
}

type TimeoutStep struct {
	Step StepConfig `json:"-"`

	// it's very tempting to make this a Duration type, but that would probably
	// prevent using `((vars))` to parameterize it
	Duration string `json:"timeout"`
}

func (step *TimeoutStep) Wrap(sub StepConfig) {
	step.Step = sub
}

func (step *TimeoutStep) Unwrap() StepConfig {
	return step.Step
}

func (step *TimeoutStep) Visit(v StepVisitor) error {
	return v.VisitTimeout(step)
}

type OnSuccessStep struct {
	Step StepConfig `json:"-"`
	Hook Step       `json:"on_success"`
}

func (step *OnSuccessStep) Wrap(sub StepConfig) {
	step.Step = sub
}

func (step *OnSuccessStep) Unwrap() StepConfig {
	return step.Step
}

func (step *OnSuccessStep) Visit(v StepVisitor) error {
	return v.VisitOnSuccess(step)
}

type OnFailureStep struct {
	Step StepConfig `json:"-"`
	Hook Step       `json:"on_failure"`
}

func (step *OnFailureStep) Wrap(sub StepConfig) {
	step.Step = sub
}

func (step *OnFailureStep) Unwrap() StepConfig {
	return step.Step
}

func (step *OnFailureStep) Visit(v StepVisitor) error {
	return v.VisitOnFailure(step)
}

type OnErrorStep struct {
	Step StepConfig `json:"-"`
	Hook Step       `json:"on_error"`
}

func (step *OnErrorStep) Wrap(sub StepConfig) {
	step.Step = sub
}

func (step *OnErrorStep) Unwrap() StepConfig {
	return step.Step
}

func (step *OnErrorStep) Visit(v StepVisitor) error {
	return v.VisitOnError(step)
}

type OnAbortStep struct {
	Step StepConfig `json:"-"`
	Hook Step       `json:"on_abort"`
}

func (step *OnAbortStep) Wrap(sub StepConfig) {
	step.Step = sub
}

func (step *OnAbortStep) Unwrap() StepConfig {
	return step.Step
}

func (step *OnAbortStep) Visit(v StepVisitor) error {
	return v.VisitOnAbort(step)
}

type EnsureStep struct {
	Step StepConfig `json:"-"`
	Hook Step       `json:"ensure"`
}

func (step *EnsureStep) Wrap(sub StepConfig) {
	step.Step = sub
}

func (step *EnsureStep) Unwrap() StepConfig {
	return step.Step
}

func (step *EnsureStep) Visit(v StepVisitor) error {
	return v.VisitEnsure(step)
}

// MaxInFlightConfig can represent either running all values in an AcrossStep
// in parallel or a applying a limit to the sub-steps that can run at once.
type MaxInFlightConfig struct {
	All   bool
	Limit int
}

const MaxInFlightAll = "all"

func (c *MaxInFlightConfig) UnmarshalJSON(version []byte) error {
	if bytes.HasPrefix(version, []byte{'"'}) {
		var data string
		err := json.Unmarshal(version, &data)
		if err != nil {
			return err
		}
		if data != MaxInFlightAll {
			return fmt.Errorf("invalid max_in_flight %q", data)
		}
		c.All = true
		return nil
	}
	err := json.Unmarshal(version, &c.Limit)
	if err != nil {
		return err
	}

	return nil
}

func (c *MaxInFlightConfig) MarshalJSON() ([]byte, error) {
	if c.All {
		return json.Marshal(MaxInFlightAll)
	}

	return json.Marshal(c.Limit)
}

// A VersionConfig represents the choice to include every version of a
// resource, the latest version of a resource, or a pinned (specific) one.
type VersionConfig struct {
	Every  bool
	Latest bool
	Pinned Version
}

func (c *VersionConfig) UnmarshalJSON(version []byte) error {
	var data interface{}

	err := json.Unmarshal(version, &data)
	if err != nil {
		return err
	}

	switch actual := data.(type) {
	case string:
		c.Every = actual == "every"
		c.Latest = actual == "latest"
	case map[string]interface{}:
		version := Version{}

		for k, v := range actual {
			if s, ok := v.(string); ok {
				version[k] = s
				continue
			}

			return fmt.Errorf("the value %v of %s is not a string", v, k)
		}

		c.Pinned = version
	default:
		return errors.New("unknown type for version")
	}

	return nil
}

const VersionLatest = "latest"
const VersionEvery = "every"

func (c *VersionConfig) MarshalJSON() ([]byte, error) {
	if c.Latest {
		return json.Marshal(VersionLatest)
	}

	if c.Every {
		return json.Marshal(VersionEvery)
	}

	if c.Pinned != nil {
		return json.Marshal(c.Pinned)
	}

	return json.Marshal("")
}

// A InputsConfig represents the choice to include every artifact within the
// job as an input to the put step or specific ones.
type InputsConfig struct {
	All       bool
	Detect    bool
	Specified []string
}

func (c *InputsConfig) UnmarshalJSON(inputs []byte) error {
	var data interface{}

	err := json.Unmarshal(inputs, &data)
	if err != nil {
		return err
	}

	switch actual := data.(type) {
	case string:
		c.All = actual == "all"
		c.Detect = actual == "detect"
	case []interface{}:
		inputs := []string{}

		for _, v := range actual {
			str, ok := v.(string)
			if !ok {
				return fmt.Errorf("non-string put input: %v", v)
			}

			inputs = append(inputs, strings.TrimSpace(str))
		}

		c.Specified = inputs
	default:
		return errors.New("unknown type for put inputs")
	}

	return nil
}

const InputsAll = "all"
const InputsDetect = "detect"

func (c InputsConfig) MarshalJSON() ([]byte, error) {
	if c.All {
		return json.Marshal(InputsAll)
	}

	if c.Detect {
		return json.Marshal(InputsDetect)
	}

	if c.Specified != nil {
		return json.Marshal(c.Specified)
	}

	return json.Marshal("")
}

func unmarshalStrict(data []byte, to interface{}) error {
	decoder := json.NewDecoder(bytes.NewBuffer(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(to)
}
