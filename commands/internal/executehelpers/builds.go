package executehelpers

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/deprecated"
	"github.com/concourse/fly/rc"
	"github.com/concourse/go-concourse/concourse"
	"github.com/tedsuo/rata"
)

func CreateBuild(
	atcRequester *deprecated.AtcRequester,
	client concourse.Client,
	privileged bool,
	inputs []Input,
	outputs []Output,
	config atc.TaskConfig,
	tags []string,
	target string,
) (atc.Build, error) {
	fact := atc.NewPlanFactory(time.Now().Unix())

	if err := config.Validate(); err != nil {
		return atc.Build{}, err
	}

	targetProps, err := rc.SelectTarget(target)
	if err != nil {
		return atc.Build{}, err
	}

	buildInputs := atc.AggregatePlan{}
	for _, input := range inputs {
		var getPlan atc.GetPlan
		if input.Path != "" {
			readPipe, err := atcRequester.CreateRequest(
				atc.ReadPipe,
				rata.Params{"pipe_id": input.Pipe.ID},
				nil,
			)
			if err != nil {
				return atc.Build{}, err
			}

			source := atc.Source{
				"uri": readPipe.URL.String(),
			}

			if targetProps.Token != nil {
				source["authorization"] = targetProps.Token.Type + " " + targetProps.Token.Value
			}

			getPlan = atc.GetPlan{
				Name:   input.Name,
				Type:   "archive",
				Source: source,
			}
		} else {
			getPlan = atc.GetPlan{
				Name:    input.Name,
				Type:    input.BuildInput.Type,
				Source:  input.BuildInput.Source,
				Version: input.BuildInput.Version,
				Params:  input.BuildInput.Params,
				Tags:    input.BuildInput.Tags,
			}
		}

		buildInputs = append(buildInputs, fact.NewPlan(getPlan))
	}

	taskPlan := fact.NewPlan(atc.TaskPlan{
		Name:       "one-off",
		Privileged: privileged,
		Config:     &config,
	})

	if len(tags) != 0 {
		taskPlan.Task.Tags = tags
	}

	buildOutputs := atc.AggregatePlan{}
	for _, output := range outputs {
		writePipe, err := atcRequester.CreateRequest(
			atc.WritePipe,
			rata.Params{"pipe_id": output.Pipe.ID},
			nil,
		)
		if err != nil {
			return atc.Build{}, err
		}
		source := atc.Source{
			"uri": writePipe.URL.String(),
		}

		params := atc.Params{
			"directory": output.Name,
		}

		if targetProps.Token != nil {
			source["authorization"] = targetProps.Token.Type + " " + targetProps.Token.Value
		}

		buildOutputs = append(buildOutputs, fact.NewPlan(atc.PutPlan{
			Name:   output.Name,
			Type:   "archive",
			Source: source,
			Params: params,
		}))
	}

	var plan atc.Plan
	if len(buildOutputs) == 0 {
		plan = fact.NewPlan(atc.DoPlan{
			fact.NewPlan(buildInputs),
			taskPlan,
		})
	} else {
		plan = fact.NewPlan(atc.DoPlan{
			fact.NewPlan(buildInputs),
			fact.NewPlan(atc.EnsurePlan{
				Step: taskPlan,
				Next: fact.NewPlan(buildOutputs),
			}),
		})
	}

	return client.CreateBuild(plan)
}
