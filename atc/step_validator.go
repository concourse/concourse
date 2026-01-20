package atc

import (
	"fmt"
	"path"
	"strings"
	"time"
)

// StepValidator is a StepVisitor which validates each step that visits it,
// collecting warnings and errors as it goes.
type StepValidator struct {
	// Warnings is a slice of warning messages to show to the user, while still
	// allowing the pipeline to be configured. This is typically used for
	// deprecations.
	//
	// This field will be populated after visiting the step.
	Warnings []ConfigWarning

	// Errors is a slice of critical errors which will prevent configuring the
	// pipeline.
	//
	// This field will be populated after visiting the step.
	Errors []string

	config  Config
	context []string

	seenGetName    scope
	localVarScopes []scope
}

type scope map[string]bool

// NewStepValidator is a constructor which initializes internal data.
//
// The Config specified is used to validate the existence of resources and jobs
// referenced by steps.
//
// The context argument contains the initial context used to annotate error and
// warning messages. For example, []string{"jobs(foo)", ".plan"} will result in
// errors like 'jobs(foo).plan.task(bar): blah blah'.
func NewStepValidator(config Config, context []string) *StepValidator {
	return &StepValidator{
		config:         config,
		context:        context,
		seenGetName:    scope{},
		localVarScopes: []scope{{}},
	}
}

func (validator *StepValidator) Validate(step Step) error {
	if len(step.UnknownFields) > 0 {
		var fieldNames []string
		for field := range step.UnknownFields {
			fieldNames = append(fieldNames, field)
		}
		validator.recordErrorf("unknown fields %+q", fieldNames)
	}

	return step.Config.Visit(validator)
}

func (validator *StepValidator) VisitTask(plan *TaskStep) error {
	validator.pushContextf(".task(%s)", plan.Name)
	defer validator.popContext()

	warning, err := ValidateIdentifier(plan.Name, validator.context...)
	if err != nil {
		validator.recordError(err.Error())
	}
	if warning != nil {
		validator.recordWarning(*warning)
	}

	if plan.Config == nil && plan.ConfigPath == "" {
		validator.recordError("must specify either `file:` or `config:`")
	}

	if plan.Config != nil && plan.ConfigPath != "" {
		validator.recordError("must specify one of `file:` or `config:`, not both")
	}

	if plan.Config != nil && (plan.Config.RootfsURI != "" || plan.Config.ImageResource != nil) && plan.ImageArtifactName != "" {
		validator.recordWarning(ConfigWarning{
			Type:    "pipeline",
			Message: validator.annotate("specifies image: on the step but also specifies an image under config: - the image: on the step takes precedence"),
		})
	}

	if plan.Hermetic {
		validator.recordWarning(ConfigWarning{
			Type:    "pipeline",
			Message: validator.annotate("specifies `hermetic:` only works against worker containerd runtime"),
		})
	}

	if plan.Config != nil {
		validator.pushContext(".config")

		if err := plan.Config.Validate(); err != nil {
			if validationErr, ok := err.(TaskValidationError); ok {
				for _, msg := range validationErr.Errors {
					validator.recordError(msg)
				}
			} else {
				validator.recordError(err.Error())
			}
		}

		validator.popContext()
	}

	return nil
}

func (validator *StepValidator) VisitGet(step *GetStep) error {
	validator.pushContextf(".get(%s)", step.Name)
	defer validator.popContext()

	warning, err := ValidateIdentifier(step.Name, validator.context...)
	if err != nil {
		validator.recordError(err.Error())
	}
	if warning != nil {
		validator.recordWarning(*warning)
	}

	if validator.seenGetName[step.Name] {
		validator.recordError("repeated name")
	}

	validator.seenGetName[step.Name] = true

	resourceName := step.ResourceName()

	_, found := validator.config.Resources.Lookup(resourceName)
	if !found {
		validator.recordErrorf("unknown resource '%s'", resourceName)
	}

	validator.pushContext(".passed")

	for _, jobGlob := range step.Passed {
		foundJob := false
		for _, jobConfig := range validator.config.Jobs {
			matched, _ := path.Match(jobGlob, jobConfig.Name)

			if matched {
				foundJob = true

				foundResource := false

				_ = jobConfig.StepConfig().Visit(StepRecursor{
					OnGet: func(input *GetStep) error {
						if input.ResourceName() == resourceName {
							foundResource = true
						}
						return nil
					},
					OnPut: func(output *PutStep) error {
						if output.ResourceName() == resourceName {
							foundResource = true
						}
						return nil
					},
				})

				if !foundResource {
					validator.recordErrorf("job '%s' does not interact with resource '%s'", jobConfig.Name, resourceName)
				}
			}
		}

		if !foundJob {
			validator.recordErrorf("no matching job(s) for '%s'", jobGlob)
		}
	}

	validator.popContext()

	return nil
}

func (validator *StepValidator) VisitPut(step *PutStep) error {
	validator.pushContextf(".put(%s)", step.Name)
	defer validator.popContext()

	warning, err := ValidateIdentifier(step.Name, validator.context...)
	if err != nil {
		validator.recordError(err.Error())
	}
	if warning != nil {
		validator.recordWarning(*warning)
	}

	resourceName := step.ResourceName()

	_, found := validator.config.Resources.Lookup(resourceName)
	if !found {
		validator.recordErrorf("unknown resource '%s'", resourceName)
	}

	return nil
}

func (validator *StepValidator) VisitRun(step *RunStep) error {
	validator.pushContextf(".run(%s.%s)", step.Type, step.Message)
	defer validator.popContext()

	warning, err := ValidateIdentifier(step.Message, validator.context...)
	if err != nil {
		validator.recordError(err.Error())
	}
	if warning != nil {
		// To prevent people from writing prototypes with invalid message
		// names, we'll explicitly error on invalid identifiers rather than
		// emitting a warning.
		validator.recordError(warning.Message)
	}

	_, found := validator.config.Prototypes.Lookup(step.Type)
	if !found {
		validator.recordErrorf("unknown prototype '%s'", step.Type)
	}

	return nil
}

func (validator *StepValidator) VisitSetPipeline(step *SetPipelineStep) error {
	validator.pushContextf(".set_pipeline(%s)", step.Name)
	defer validator.popContext()

	warning, err := ValidateIdentifier(step.Name, validator.context...)
	if err != nil {
		validator.recordError(err.Error())
	}
	if warning != nil {
		validator.recordWarning(*warning)
	}

	if step.File == "" {
		validator.recordError("no file specified")
	}

	return nil
}

func (validator *StepValidator) VisitLoadVar(step *LoadVarStep) error {
	validator.pushContextf(".load_var(%s)", step.Name)
	defer validator.popContext()

	warning, err := ValidateIdentifier(step.Name, validator.context...)
	if err != nil {
		validator.recordError(err.Error())
	}
	if warning != nil {
		validator.recordWarning(*warning)
	}

	validator.declareLocalVar(step.Name)

	if step.File == "" {
		validator.recordError("no file specified")
	}

	return nil
}

func (validator *StepValidator) VisitTry(step *TryStep) error {
	validator.pushContext(".try")
	defer validator.popContext()

	return validator.Validate(step.Step)
}

func (validator *StepValidator) VisitDo(step *DoStep) error {
	validator.pushContext(".do")
	defer validator.popContext()

	for i, sub := range step.Steps {
		validator.pushContextf("[%d]", i)

		err := validator.Validate(sub)
		if err != nil {
			return err
		}

		validator.popContext()
	}

	return nil
}

func (validator *StepValidator) VisitInParallel(step *InParallelStep) error {
	validator.pushContext(".in_parallel")
	defer validator.popContext()

	for i, sub := range step.Config.Steps {
		validator.pushContextf(".steps[%d]", i)

		err := validator.Validate(sub)
		if err != nil {
			return err
		}

		validator.popContext()
	}

	return nil
}

func (validator *StepValidator) VisitAcross(step *AcrossStep) error {
	validator.pushContext(".across")
	defer validator.popContext()

	validator.pushLocalVarScope()
	defer validator.popLocalVarScope()

	if len(step.Vars) == 0 {
		validator.recordError("no vars specified")
	}

	for i, v := range step.Vars {
		validator.pushContextf("[%d]", i)

		validator.declareLocalVar(v.Var)

		validator.pushContext(".max_in_flight")
		if v.MaxInFlight != nil && !v.MaxInFlight.All && v.MaxInFlight.Limit <= 0 {
			validator.recordError("must be greater than 0")
		}
		validator.popContext()
		validator.popContext()
	}

	return step.Step.Visit(validator)
}

func (validator *StepValidator) VisitTimeout(step *TimeoutStep) error {
	err := step.Step.Visit(validator)
	if err != nil {
		return err
	}

	validator.pushContext(".timeout")
	defer validator.popContext()

	_, err = time.ParseDuration(step.Duration)
	if err != nil {
		validator.recordErrorf("invalid duration '%s'", step.Duration)
	}

	return nil
}

func (validator *StepValidator) VisitRetry(step *RetryStep) error {
	err := step.Step.Visit(validator)
	if err != nil {
		return err
	}

	validator.pushContext(".attempts")
	defer validator.popContext()

	if step.Attempts <= 0 {
		validator.recordError("must be greater than 0")
	}

	return nil
}

func (validator *StepValidator) VisitOnSuccess(step *OnSuccessStep) error {
	err := step.Step.Visit(validator)
	if err != nil {
		return err
	}

	validator.pushContext(".on_success")
	defer validator.popContext()

	return validator.Validate(step.Hook)
}

func (validator *StepValidator) VisitOnFailure(step *OnFailureStep) error {
	err := step.Step.Visit(validator)
	if err != nil {
		return err
	}

	validator.pushContext(".on_failure")
	defer validator.popContext()

	return validator.Validate(step.Hook)
}

func (validator *StepValidator) VisitOnAbort(step *OnAbortStep) error {
	err := step.Step.Visit(validator)
	if err != nil {
		return err
	}

	validator.pushContext(".on_abort")
	defer validator.popContext()

	return validator.Validate(step.Hook)
}

func (validator *StepValidator) VisitOnError(step *OnErrorStep) error {
	err := step.Step.Visit(validator)
	if err != nil {
		return err
	}

	validator.pushContext(".on_error")
	defer validator.popContext()

	return validator.Validate(step.Hook)
}

func (validator *StepValidator) VisitEnsure(step *EnsureStep) error {
	err := step.Step.Visit(validator)
	if err != nil {
		return err
	}

	validator.pushContext(".ensure")
	defer validator.popContext()

	return validator.Validate(step.Hook)
}

func (validator *StepValidator) recordWarning(warning ConfigWarning) {
	validator.Warnings = append(validator.Warnings, warning)
}

func (validator *StepValidator) recordError(message string) {
	validator.Errors = append(validator.Errors, validator.annotate(message))
}

func (validator *StepValidator) recordErrorf(message string, args ...any) {
	validator.Errors = append(validator.Errors, validator.annotate(fmt.Sprintf(message, args...)))
}

func (validator *StepValidator) annotate(message string) string {
	return fmt.Sprintf("%s: %s", strings.Join(validator.context, ""), message)
}

func (validator *StepValidator) pushContext(ctx string) {
	validator.context = append(validator.context, ctx)
}

func (validator *StepValidator) pushContextf(ctx string, args ...any) {
	validator.context = append(validator.context, fmt.Sprintf(ctx, args...))
}

func (validator *StepValidator) popContext() {
	validator.context = validator.context[0 : len(validator.context)-1]
}

func (validator *StepValidator) pushLocalVarScope() {
	validator.localVarScopes = append(validator.localVarScopes, scope{})
}

func (validator *StepValidator) popLocalVarScope() {
	validator.localVarScopes = validator.localVarScopes[0 : len(validator.localVarScopes)-1]
}

func (validator *StepValidator) currentLocalVarScope() scope {
	return validator.localVarScopes[len(validator.localVarScopes)-1]
}

func (validator *StepValidator) localVarIsDeclared(name string) bool {
	for _, scope := range validator.localVarScopes {
		if scope[name] {
			return true
		}
	}
	return false
}

func (validator *StepValidator) declareLocalVar(name string) {
	if validator.currentLocalVarScope()[name] {
		validator.recordError("repeated var name")
	} else if validator.localVarIsDeclared(name) {
		validator.recordWarning(ConfigWarning{
			Type:    "var_shadowed",
			Message: validator.annotate(fmt.Sprintf("shadows local var '%s'", name)),
		})
	}

	validator.currentLocalVarScope()[name] = true
}
