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
	pf := atc.NewPlanFactory(time.Now().Unix())

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

		buildInputs = append(buildInputs, pf.NewPlan(getPlan))
	}

	taskPlan := pf.NewPlan(atc.TaskPlan{
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

		buildOutputs = append(buildOutputs, pf.NewPlan(atc.PutPlan{
			Name:   output.Name,
			Type:   "archive",
			Source: source,
			Params: params,
		}))
	}

	var plan atc.Plan
	if len(buildOutputs) == 0 {
		plan = pf.NewPlan(atc.OnSuccessPlan{
			Step: pf.NewPlan(buildInputs),
			Next: taskPlan,
		})
	} else {
		plan = pf.NewPlan(atc.OnSuccessPlan{
			Step: pf.NewPlan(buildInputs),
			Next: pf.NewPlan(atc.EnsurePlan{
				Step: taskPlan,
				Next: pf.NewPlan(buildOutputs),
			}),
		})
	}

	return client.CreateBuild(plan)
}
