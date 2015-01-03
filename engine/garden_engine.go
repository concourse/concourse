package engine

import (
	"encoding/json"
	"io"
	"os"

	garden "github.com/cloudfoundry-incubator/garden/api"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
)

type GardenMetadata struct {
	Plan atc.BuildPlan
	// Progress GardenProgress
}

//
// type GardenProgress struct {
// 	Inputs  []InputSnapshot
// 	Build   *BuildSnapshot
// 	Outputs []OutputSnapshot
// }
//
// type InputSnapshot struct {
// 	InputPlan      atc.InputPlan
// 	InputProcessID int
// }
//
// type BuildSnapshot struct {
// 	BuildConfig    atc.BuildConfig
// 	BuildProcessID int
// }
//
// type OutputSnapshot struct {
// 	OutputPlan      atc.OutputPlan
// 	OutputProcessID int
// }

type gardenEngine struct {
	factory exec.Factory
}

func NewGardenEngine(factory exec.Factory) Engine {
	return &gardenEngine{factory: factory}
}

func (engine *gardenEngine) Name() string {
	return "garden.v1"
}

func (engine *gardenEngine) CreateBuild(model db.Build, plan atc.BuildPlan) (Build, error) {
	return &gardenBuild{
		factory: engine.factory,
		metadata: GardenMetadata{
			Plan: plan,
		},
	}, nil
}

func (engine *gardenEngine) LookupBuild(model db.Build) (Build, error) {
	var metadata GardenMetadata
	err := json.Unmarshal([]byte(model.EngineMetadata), &metadata)
	if err != nil {
		return nil, err
	}

	return &gardenBuild{
		factory:  engine.factory,
		metadata: metadata,
	}, nil
}

type gardenBuild struct {
	factory exec.Factory

	signals chan os.Signal

	metadata GardenMetadata
}

func (build *gardenBuild) Metadata() string {
	payload, err := json.Marshal(build.metadata)
	if err != nil {
		panic("failed to marshal build metadata: " + err.Error())
	}

	return string(payload)
}

func (build *gardenBuild) Abort() error {
	return nil
}

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
func (build *gardenBuild) Resume(lager.Logger) error {
	plan := build.metadata.Plan

	var step exec.Step

	if len(plan.Inputs) > 0 {
		inputs := exec.Aggregate{}

		for _, input := range plan.Inputs {
			ioConfig := exec.IOConfig{
				Stdout: build.ioWriter(event.Origin{
					Type: event.OriginTypeInput,
					Name: input.Name,
				}),
				Stderr: build.ioWriter(event.Origin{
					Type: event.OriginTypeInput,
					Name: input.Name,
				}),
			}

			resourceConfig := atc.ResourceConfig{
				Name:   input.Resource,
				Type:   input.Type,
				Source: input.Source,
			}

			inputs[input.Name] = build.factory.Get(ioConfig, resourceConfig, input.Params, input.Version)
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

	var successReporter exec.SuccessReporter

	if configSource != nil {
		ioConfig := exec.IOConfig{
			Stdout: build.ioWriter(event.Origin{
				Type: event.OriginTypeRun,
				Name: "stdout",
			}),
			Stderr: build.ioWriter(event.Origin{
				Type: event.OriginTypeRun,
				Name: "stderr",
			}),
		}

		configSource = initializeEmitter{configSource}

		step = exec.Compose(step, successReporter.Subject(build.factory.Execute(ioConfig, configSource)))
		// finish event? (should probably have an origin.)
	}

	if len(plan.Outputs) > 0 {
		outputs := exec.Aggregate{}

		for _, output := range plan.Outputs {
			ioConfig := exec.IOConfig{
				Stdout: build.ioWriter(event.Origin{
					Type: event.OriginTypeOutput,
					Name: output.Name,
				}),
				Stderr: build.ioWriter(event.Origin{
					Type: event.OriginTypeOutput,
					Name: output.Name,
				}),
			}

			resourceConfig := atc.ResourceConfig{
				Name:   output.Name,
				Type:   output.Type,
				Source: output.Source,
			}

			outputs[output.Name] = exec.Conditional{
				Conditions: output.On,
				Step:       build.factory.Put(ioConfig, resourceConfig, output.Params),
			}
		}

		step = exec.Compose(step, outputs)
	}

	source := step.Using(&exec.NoopArtifactSource{})

	// start

	process := ifrit.Invoke(source)

	exited := process.Wait()

	var aborted bool

	for {
		select {
		case err := <-exited:
			if aborted {
				// aborted
			} else if err != nil {
				// errored
			} else if successReporter.Successful() {
				// succeeded
			} else {
				// failed
			}

			return nil

		case sig := <-build.signals:
			process.Signal(sig)

			if sig == os.Kill {
				aborted = true
			}
		}
	}
}

func (build *gardenBuild) Hijack(garden.ProcessSpec, garden.ProcessIO) (garden.Process, error) {
	return nil, nil
}

func (build *gardenBuild) ioWriter(origin event.Origin) io.Writer {
	return nil
}

type initializeEmitter struct {
	exec.BuildConfigSource
}

func (emitter initializeEmitter) FetchConfig(source exec.ArtifactSource) (atc.BuildConfig, error) {
	config, err := emitter.BuildConfigSource.FetchConfig(source)
	if err != nil {
		return atc.BuildConfig{}, err
	}

	// initialize event

	return config, nil
}
