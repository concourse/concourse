package executehelpers

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/rc"
)

func CreateBuildPlan(
	fact atc.PlanFactory,
	target rc.Target,
	privileged bool,
	inputs []Input,
	inputMappings map[string]string,
	resourceTypes atc.ResourceTypes,
	outputs []Output,
	config atc.TaskConfig,
	tags []string,
) (atc.Plan, error) {
	if err := config.Validate(); err != nil {
		return atc.Plan{}, err
	}

	buildInputs := atc.InParallelPlan{}
	for _, input := range inputs {
		buildInputs.Steps = append(buildInputs.Steps, input.Plan)
	}

	taskPlan := fact.NewPlan(atc.TaskPlan{
		Name:          "one-off",
		Privileged:    privileged,
		Config:        &config,
		InputMapping:  inputMappings,
		ResourceTypes: resourceTypes,
	})

	if len(tags) != 0 {
		taskPlan.Task.Tags = tags
	}

	buildOutputs := atc.InParallelPlan{}
	for _, output := range outputs {
		buildOutputs.Steps = append(buildOutputs.Steps, output.Plan)
	}

	var plan atc.Plan
	if len(buildOutputs.Steps) == 0 {
		plan = fact.NewPlan(atc.DoPlan{
			fact.NewPlan(buildInputs),
			taskPlan,
		})
	} else {
		plan = fact.NewPlan(atc.EnsurePlan{
			Step: fact.NewPlan(atc.DoPlan{
				fact.NewPlan(buildInputs),
				taskPlan,
			}),
			Next: fact.NewPlan(buildOutputs),
		})
	}

	return plan, nil
}
