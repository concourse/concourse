package plan

import (
	"fmt"

	"github.com/concourse/atc"
	uuid "github.com/nu7hatch/gouuid"
)

type MissingJobError struct {
	JobName string
}

func (e *MissingJobError) Error() string {
	return fmt.Sprintf("job '%s' is not present in pipeline config", e.JobName)
}

type MissingResourceError struct {
	ResourceName string
}

func (e *MissingResourceError) Error() string {
	return fmt.Sprintf("resource '%s' is not present in pipeline config", e.ResourceName)
}

// Planner takes in a pipeline config and generates a plan
// Generates a check plan and the execution plan.
// The check plan must be run before the execution plan, and the versions gathered
// via the check plan are substituted as the version in the execution plan.
type Planner struct{}

func (p *Planner) GenerateCheckPlanForPipeline(pipelineConfig atc.Config) (Plan, error) {
	return Plan{}, nil
}

func (p *Planner) GenerateCheckPlanForJob(pipelineConfig atc.Config, jobName string) (Plan, error) {
	jobConfig, found := pipelineConfig.Jobs.Lookup(jobName)
	if !found {
		return Plan{}, &MissingJobError{JobName: jobName}
	}

	checkSteps, err := constructCheckStepsForPlanSequence(jobConfig.Plan, pipelineConfig.Resources, pipelineConfig.ResourceTypes)
	if err != nil {
		return Plan{}, err
	}

	return Plan{
		Steps: checkSteps,
	}, nil
}

func (p *Planner) GenerateExecutionPlanForJob(pipelineConfig atc.Config, jobName string) (Plan, error) {
	jobConfig, found := pipelineConfig.Jobs.Lookup(jobName)
	if !found {
		return Plan{}, &MissingJobError{JobName: jobName}
	}

	planSequence := jobConfig.Plan
	for _, planConfig := range planSequence {
		switch {
		case planConfig.Do != nil:
		case planConfig.Put != "":
		case planConfig.Get != "":
		case planConfig.Task != "":
		case planConfig.Try != nil:
		case planConfig.Aggregate != nil:
		}
	}

	return Plan{}, nil
}

func constructCheckStepsForPlanSequence(planSequence atc.PlanSequence, resources atc.ResourceConfigs, resourceTypes atc.ResourceTypes) ([]interface{}, error) {
	var checkSteps []interface{}

	for _, planConfig := range planSequence {
		switch {
		case planConfig.Do != nil:
			c, err := constructCheckStepsForPlanSequence(*planConfig.Do, resources, resourceTypes)
			if err != nil {
				return nil, err
			}
			checkSteps = append(checkSteps, c...)
		case planConfig.Get != "":
			resourceName := planConfig.Get
			resourceConfig, found := resources.Lookup(resourceName)
			if !found {
				return nil, &MissingResourceError{ResourceName: resourceName}
			}

			actions, err := generateCheckActions(resourceConfig.Type, resourceConfig.Source, resourceTypes)
			if err != nil {
				return nil, err
			}

			checkSteps = append(checkSteps, CheckStep{
				Actions: actions,
			})
		case planConfig.Try != nil:

		case planConfig.Aggregate != nil:
			c, err := constructCheckStepsForPlanSequence(*planConfig.Aggregate, resources, resourceTypes)
			if err != nil {
				return nil, err
			}
			checkSteps = append(checkSteps, c...)
		}
	}

	return checkSteps, nil
}

func generateCheckActions(
	resourceType string,
	resourceSource atc.Source,
	resourceTypes atc.ResourceTypes,
) ([]interface{}, error) {
	var actions []interface{}

	for {
		resourceTypeConfig, found := resourceTypes.Lookup(resourceType)
		if !found {
			// using base resource type
			actions = append(
				[]interface{}{
					CheckAction{
						RootFSSource: BaseResourceTypeRootFSSource{
							Name: resourceType,
						},
						Source: resourceSource,
					},
				},
				actions...,
			)

			break
		}

		generatedOutputName, err := uuid.NewV4()
		if err != nil {
			return nil, err
		}

		actions = append(
			[]interface{}{
				GetAction{
					Source: resourceTypeConfig.Source,
					Outputs: []Output{
						{Name: generatedOutputName.String()},
					},
				},
				CheckAction{
					RootFSSource: OutputRootFSSource{
						Name: generatedOutputName.String(),
					},
					Source: resourceSource,
				},
			},
			actions...,
		)

		resourceTypes = resourceTypes.Without(resourceType)
		resourceType = resourceTypeConfig.Type
		resourceSource = resourceTypeConfig.Source
	}

	var previousCheckAction CheckAction

	for i, action := range actions {
		if checkAction, ok := action.(CheckAction); ok {
			previousCheckAction = checkAction
		}
		if getAction, ok := action.(GetAction); ok {
			getAction.RootFSSource = previousCheckAction.RootFSSource
			actions[i] = getAction
		}
	}

	return actions, nil
}
