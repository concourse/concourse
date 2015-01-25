package engine

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/exec"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
)

type execMetadata struct {
	Plan atc.BuildPlan
}

type execEngine struct {
	factory         exec.Factory
	delegateFactory BuildDelegateFactory
	db              EngineDB
}

func NewExecEngine(factory exec.Factory, delegateFactory BuildDelegateFactory, db EngineDB) Engine {
	return &execEngine{
		factory:         factory,
		delegateFactory: delegateFactory,
		db:              db,
	}
}

func (engine *execEngine) Name() string {
	return "exec.v1"
}

func (engine *execEngine) CreateBuild(model db.Build, plan atc.BuildPlan) (Build, error) {
	return &execBuild{
		buildID:  model.ID,
		db:       engine.db,
		factory:  engine.factory,
		delegate: engine.delegateFactory.Delegate(model.ID),
		metadata: execMetadata{
			Plan: plan,
		},

		signals: make(chan os.Signal, 1),
	}, nil
}

func (engine *execEngine) LookupBuild(model db.Build) (Build, error) {
	var metadata execMetadata
	err := json.Unmarshal([]byte(model.EngineMetadata), &metadata)
	if err != nil {
		return nil, err
	}

	return &execBuild{
		buildID:  model.ID,
		db:       engine.db,
		factory:  engine.factory,
		delegate: engine.delegateFactory.Delegate(model.ID),
		metadata: metadata,

		signals: make(chan os.Signal, 1),
	}, nil
}

type execBuild struct {
	buildID int
	db      EngineDB

	factory  exec.Factory
	delegate BuildDelegate

	signals chan os.Signal

	metadata execMetadata
}

func (build *execBuild) Metadata() string {
	payload, err := json.Marshal(build.metadata)
	if err != nil {
		panic("failed to marshal build metadata: " + err.Error())
	}

	return string(payload)
}

func (build *execBuild) Abort() error {
	build.signals <- os.Kill
	return nil
}

func (build *execBuild) Resume(logger lager.Logger) {
	step := exec.Compose(
		build.aggregateInputsStep(logger.Session("inputs")),
		exec.Compose(
			build.executeStep(logger.Session("execute")),
			build.aggregateOutputsStep(logger.Session("outputs")),
		),
	)

	source := step.Using(&exec.NoopArtifactSource{})

	process := ifrit.Background(source)

	exited := process.Wait()

	for {
		select {
		case err := <-exited:
			build.delegate.Finish(logger.Session("finish"), err)
			return

		case sig := <-build.signals:
			process.Signal(sig)

			if sig == os.Kill {
				build.delegate.Aborted(logger)
			}
		}
	}
}

func (build *execBuild) Hijack(spec atc.HijackProcessSpec, io HijackProcessIO) (HijackedProcess, error) {
	ioConfig := exec.IOConfig{
		Stdin:  io.Stdin,
		Stdout: io.Stdout,
		Stderr: io.Stderr,
	}

	return build.factory.Hijack(build.executeSessionID(), ioConfig, spec)
}

func (build *execBuild) aggregateInputsStep(logger lager.Logger) exec.Step {
	inputs := exec.Aggregate{}

	for _, input := range build.metadata.Plan.Inputs {
		inputs[input.Name] = build.factory.Get(
			build.inputSessionID(input.Name),
			build.delegate.InputDelegate(logger.Session(input.Name), input),
			atc.ResourceConfig{
				Name:   input.Resource,
				Type:   input.Type,
				Source: input.Source,
			},
			input.Params,
			input.Version,
		)
	}

	return inputs
}

func (build *execBuild) executeStep(logger lager.Logger) exec.Step {
	plan := build.metadata.Plan

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
	} else {
		return exec.Identity{}
	}

	return build.factory.Execute(
		build.executeSessionID(),
		build.delegate.ExecutionDelegate(logger),
		exec.Privileged(plan.Privileged),
		configSource,
	)
}

func (build *execBuild) aggregateOutputsStep(logger lager.Logger) exec.Step {
	plan := build.metadata.Plan

	outputs := exec.Aggregate{}

	for _, output := range plan.Outputs {
		step := build.factory.Put(
			build.outputSessionID(output.Name),
			build.delegate.OutputDelegate(logger.Session(output.Name), output),
			atc.ResourceConfig{
				Name:   output.Name,
				Type:   output.Type,
				Source: output.Source,
			},
			output.Params,
		)

		if plan.Config != nil || plan.ConfigPath != "" {
			// if there's a build configured, make this conditional
			step = exec.Conditional{
				Conditions: output.On,
				Step:       step,
			}
		}

		outputs[output.Name] = step
	}

	return outputs
}

func (build *execBuild) executeSessionID() exec.SessionID {
	return exec.SessionID(fmt.Sprintf("build-%d-execute", build.buildID))
}

func (build *execBuild) inputSessionID(inputName string) exec.SessionID {
	return exec.SessionID(fmt.Sprintf("build-%d-input-%s", build.buildID, inputName))
}

func (build *execBuild) outputSessionID(outputName string) exec.SessionID {
	return exec.SessionID(fmt.Sprintf("build-%d-output-%s", build.buildID, outputName))
}
