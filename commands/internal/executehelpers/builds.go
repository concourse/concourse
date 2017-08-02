package executehelpers

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
)

func CreateBuildPlan(
	target rc.Target,
	privileged bool,
	inputs []Input,
	outputs []Output,
	config atc.TaskConfig,
	tags []string,
) (atc.Plan, error) {
	fact := atc.NewPlanFactory(time.Now().Unix())

	if err := config.Validate(); err != nil {
		return atc.Plan{}, err
	}

	buildInputs := atc.AggregatePlan{}
	for _, input := range inputs {
		var getPlan atc.GetPlan
		if input.Path != "" {
			source := atc.Source{
				"uri": input.Pipe.ReadURL,
			}

			if target.CACert() != "" {
				source["ca_cert"] = target.CACert()
			}

			tlsconfig := target.TLSConfig()
			if tlsconfig != nil && tlsconfig.InsecureSkipVerify {
				source["skip_ssl_validation"] = true
			}

			if auth, ok := target.TokenAuthorization(); ok {
				source["authorization"] = auth
			}

			getPlan = atc.GetPlan{
				Name:   input.Name,
				Type:   "archive",
				Source: source,
			}
		} else {
			version := input.BuildInput.Version
			getPlan = atc.GetPlan{
				Name:    input.Name,
				Type:    input.BuildInput.Type,
				Source:  input.BuildInput.Source,
				Version: &version,
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
		source := atc.Source{
			"uri": output.Pipe.ReadURL,
		}

		params := atc.Params{
			"directory": output.Name,
		}

		if target.CACert() != "" {
			source["ca_cert"] = target.CACert()
		}

		tlsconfig := target.TLSConfig()
		if tlsconfig != nil && tlsconfig.InsecureSkipVerify {
			source["skip_ssl_validation"] = true
		}

		if auth, ok := target.TokenAuthorization(); ok {
			source["authorization"] = auth
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
