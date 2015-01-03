package engine

import (
	garden "github.com/cloudfoundry-incubator/garden/api"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/exec"
	"github.com/pivotal-golang/lager"
)

// Compose(
//   Compose(
//     Aggregate{ // inputs
//       "A": Get()
//       "B": Get()
//     },
//     Execute( // build
//       BuildConfigSource,
//     ),
//   ),
//   Aggregate{ // outputs
//     "C": Conditional{Put()},
//     "D": Conditional{Put()},
//   },
// )
func foo(factory exec.Factory, plan atc.BuildPlan) exec.Step {
	var step exec.Step

	if len(plan.Inputs) > 0 {
		inputs := exec.Aggregate{}

		for _, input := range plan.Inputs {
			resourceConfig := atc.ResourceConfig{
				Name:   input.Resource,
				Type:   input.Type,
				Source: input.Source,
			}

			inputs[input.Name] = factory.Get(resourceConfig, input.Params, input.Version)
		}

		step = inputs
	}

	var configSource exec.BuildConfigSource
	if plan.Config != nil && plan.ConfigPath != "" {
		configSource = exec.MergedConfigSource{
			A: exec.FileConfigSource{plan.ConfigPath},
			B: exec.StaticConfigSource{*plan.Config},
		}
	} else if plan.Config != nil {
		configSource = exec.StaticConfigSource{*plan.Config}
	} else if plan.ConfigPath != "" {
		configSource = exec.FileConfigSource{plan.ConfigPath}
	}

	if configSource != nil {
		step = exec.Compose(step, factory.Execute(configSource))
	}

	if len(plan.Outputs) > 0 {
		outputs := exec.Aggregate{}

		for _, output := range plan.Outputs {
			resourceConfig := atc.ResourceConfig{
				Name:   output.Name,
				Type:   output.Type,
				Source: output.Source,
			}

			outputs[output.Name] = exec.Conditional{
				Conditions: output.On,
				Step:       factory.Put(resourceConfig, output.Params),
			}
		}

		step = exec.Compose(step, outputs)
	}

	return step
}

type GardenMetadata struct {
	Plan     atc.BuildPlan
	Progress GardenProgress
}

type GardenProgress struct {
	Inputs  []InputSnapshot
	Build   *BuildSnapshot
	Outputs []OutputSnapshot
}

type InputSnapshot struct {
	InputPlan      atc.InputPlan
	InputProcessID int
}

type BuildSnapshot struct {
	BuildConfig    atc.BuildConfig
	BuildProcessID int
}

type OutputSnapshot struct {
	OutputPlan      atc.OutputPlan
	OutputProcessID int
}

type gardenEngine struct {
	gardenClient garden.Client
}

func (engine *gardenEngine) Name() string {
	return "garden.v1"
}

func (engine *gardenEngine) CreateBuild(model db.Build, plan atc.BuildPlan) (Build, error) {
	return nil, nil
}

func (engine *gardenEngine) LookupBuild(model db.Build) (Build, error) {
	return nil, nil
}

type gardenBuild struct {
	gardenClient garden.Client
}

func (build *gardenBuild) Metadata() string {
	return ""
}

func (build *gardenBuild) Abort() error {
	return nil
}

func (build *gardenBuild) Resume(lager.Logger) error {
	return nil
}

func (build *gardenBuild) Hijack(garden.ProcessSpec, garden.ProcessIO) (garden.Process, error) {
	return nil, nil
}

//
// type Input interface {
// 	Fetch(atc.InputPlan) (ifrit.Process, error)
//
// 	Version() atc.Version
//
// 	StreamOut() (io.Reader, error)
// }
//
// type Step interface {
// 	Recover([]byte) error
// 	Snapshot() []byte
//
// 	Result(interface{}) error
//
// 	ifrit.Runner
// }
//
// type inputStep struct {
// 	gardenClient garden.Client
//
// 	buildGuid string
// 	inputPlan atc.InputPlan
//
// 	processID *int
// }
//
// func (step *inputStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
// 	if step.processID == nil {
// 		// create container; start fetching
// 	}
//
// 	close(ready)
//
// 	// continue fetching
//
// 	return nil
// }
