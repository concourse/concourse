package atc

import (
	"fmt"
	"strings"
	"time"
)

type StepValidator struct {
	config  Config
	context []string

	seenGetName     map[string]bool
	seenLoadVarName map[string]bool

	Warnings []string
	Errors   []string
}

func NewStepValidator(config Config, context []string) *StepValidator {
	return &StepValidator{
		config:          config,
		context:         context,
		seenGetName:     map[string]bool{},
		seenLoadVarName: map[string]bool{},
	}
}

func (validator *StepValidator) recordWarning(message string, args ...interface{}) {
	validator.Warnings = append(validator.Warnings, validator.annotate(fmt.Sprintf(message, args...)))
}

func (validator *StepValidator) recordError(message string, args ...interface{}) {
	validator.Errors = append(validator.Errors, validator.annotate(fmt.Sprintf(message, args...)))
}

func (validator *StepValidator) annotate(message string) string {
	return fmt.Sprintf("%s: %s", strings.Join(validator.context, ""), message)
}

func (validator *StepValidator) pushContext(ctx string, args ...interface{}) {
	validator.context = append(validator.context, fmt.Sprintf(ctx, args...))
}

func (validator *StepValidator) popContext() {
	validator.context = validator.context[0 : len(validator.context)-1]
}

func (validator *StepValidator) VisitTask(plan *TaskStep) error {
	validator.pushContext(fmt.Sprintf("task(%s)", plan.Name))
	defer validator.popContext()

	if plan.Config == nil && plan.ConfigPath == "" {
		validator.recordError("must specify either file: or config:")
	}

	if plan.Config != nil && plan.ConfigPath != "" {
		validator.recordError("must specify one of file: or config:, not both")
	}

	if plan.Config != nil && (plan.Config.RootfsURI != "" || plan.Config.ImageResource != nil) && plan.ImageArtifactName != "" {
		validator.recordWarning("specifies image: on the step but also specifies an image under config: - the image: on the step takes precedence")
	}

	if plan.Config != nil {
		validator.pushContext("config")

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
	validator.pushContext(fmt.Sprintf(".get(%s)", step.Name))
	defer validator.popContext()

	if validator.seenGetName[step.Name] {
		validator.recordError("repeated name")
	}

	validator.seenGetName[step.Name] = true

	resourceName := step.ResourceName()

	_, found := validator.config.Resources.Lookup(resourceName)
	if !found {
		validator.recordError("unknown resource: %s", step.Resource)
	}

	validator.pushContext(".passed")

	for _, job := range step.Passed {
		jobConfig, found := validator.config.Jobs.Lookup(job)
		if !found {
			validator.recordError("unknown job: %s", job)
			continue
		}

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
			validator.recordError("job '%s' does not interact with resource '%s'", job, resourceName)
		}
	}

	validator.popContext()

	return nil
}

func (validator *StepValidator) VisitPut(step *PutStep) error {
	validator.pushContext(".put(%s)", step.Name)
	defer validator.popContext()

	resourceName := step.ResourceName()

	_, found := validator.config.Resources.Lookup(resourceName)
	if !found {
		validator.recordError("unknown resource: %s", step.Resource)
	}

	return nil
}

func (validator *StepValidator) VisitSetPipeline(step *SetPipelineStep) error {
	validator.pushContext(".set_pipeline(%s)", step.Name)
	defer validator.popContext()

	if step.File == "" {
		validator.recordError("no file specified")
	}

	return nil
}

func (validator *StepValidator) VisitLoadVar(step *LoadVarStep) error {
	validator.pushContext(".load_var(%s)", step.Name)
	defer validator.popContext()

	if validator.seenLoadVarName[step.Name] {
		validator.recordError("repeated name")
	}

	validator.seenLoadVarName[step.Name] = true

	if step.File == "" {
		validator.recordError("no file specified")
	}

	return nil
}

func (validator *StepValidator) VisitTry(step *TryStep) error {
	validator.pushContext(".try")
	defer validator.popContext()
	return step.Step.Config.Visit(validator)
}

func (validator *StepValidator) VisitDo(step *DoStep) error {
	validator.pushContext(".do")
	defer validator.popContext()

	for i, sub := range step.Steps {
		validator.pushContext(fmt.Sprintf("[%d]", i))

		err := sub.Config.Visit(validator)
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
		validator.pushContext(".steps[%d]", i)

		err := sub.Config.Visit(validator)
		if err != nil {
			return err
		}

		validator.popContext()
	}

	return nil
}

func (validator *StepValidator) VisitAggregate(step *AggregateStep) error {
	validator.pushContext(".aggregate")
	defer validator.popContext()

	validator.recordWarning("the aggregate step is deprecated and will be removed in a future version")

	for i, sub := range step.Steps {
		validator.pushContext("[%d]", i)

		err := sub.Config.Visit(validator)
		if err != nil {
			return err
		}

		validator.popContext()
	}

	return nil
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
		validator.recordError("invalid duration: %s", err)
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

	if step.Attempts < 0 {
		validator.recordError("cannot be negative")
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

	return step.Hook.Config.Visit(validator)
}

func (validator *StepValidator) VisitOnFailure(step *OnFailureStep) error {
	err := step.Step.Visit(validator)
	if err != nil {
		return err
	}

	validator.pushContext(".on_failure")
	defer validator.popContext()

	return step.Hook.Config.Visit(validator)
}

func (validator *StepValidator) VisitOnAbort(step *OnAbortStep) error {
	err := step.Step.Visit(validator)
	if err != nil {
		return err
	}

	validator.pushContext(".on_abort")
	defer validator.popContext()

	return step.Hook.Config.Visit(validator)
}

func (validator *StepValidator) VisitOnError(step *OnErrorStep) error {
	err := step.Step.Visit(validator)
	if err != nil {
		return err
	}

	validator.pushContext(".on_error")
	defer validator.popContext()

	return step.Hook.Config.Visit(validator)
}

func (validator *StepValidator) VisitEnsure(step *EnsureStep) error {
	err := step.Step.Visit(validator)
	if err != nil {
		return err
	}

	validator.pushContext(".ensure")
	defer validator.popContext()

	return step.Hook.Config.Visit(validator)
}
