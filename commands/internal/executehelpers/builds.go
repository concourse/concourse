package executehelpers

import (
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
	target string,
) (atc.Build, error) {
	if err := config.Validate(); err != nil {
		return atc.Build{}, err
	}

	targetProps, err := rc.SelectTarget(target)
	if err != nil {
		return atc.Build{}, err
	}

	buildInputs := atc.AggregatePlan{}
	for i, input := range inputs {
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

		buildInputs = append(buildInputs, atc.Plan{
			Location: &atc.Location{
				// offset by 2 because aggregate gets parallelgroup ID 1
				ID:            uint(i) + 2,
				ParentID:      0,
				ParallelGroup: 1,
			},
			Get: &getPlan,
		})
	}

	taskPlan := atc.Plan{
		Location: &atc.Location{
			// offset by 1 because aggregate gets parallelgroup ID 1
			ID:       uint(len(inputs)) + 2,
			ParentID: 0,
		},
		Task: &atc.TaskPlan{
			Name:       "one-off",
			Privileged: privileged,
			Config:     &config,
		},
	}

	buildOutputs := atc.AggregatePlan{}
	for i, output := range outputs {
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

		buildOutputs = append(buildOutputs, atc.Plan{
			Location: &atc.Location{
				ID:            taskPlan.Location.ID + 2 + uint(i),
				ParentID:      0,
				ParallelGroup: taskPlan.Location.ID + 1,
			},
			Put: &atc.PutPlan{
				Name:   output.Name,
				Type:   "archive",
				Source: source,
				Params: params,
			},
		})
	}

	var plan atc.Plan
	if len(buildOutputs) == 0 {
		plan = atc.Plan{
			OnSuccess: &atc.OnSuccessPlan{
				Step: atc.Plan{
					Aggregate: &buildInputs,
				},
				Next: taskPlan,
			},
		}
	} else {
		plan = atc.Plan{
			OnSuccess: &atc.OnSuccessPlan{
				Step: atc.Plan{
					Aggregate: &buildInputs,
				},
				Next: atc.Plan{
					Ensure: &atc.EnsurePlan{
						Step: taskPlan,
						Next: atc.Plan{
							Aggregate: &buildOutputs,
						},
					},
				},
			},
		}
	}

	return client.CreateBuild(plan)
}
