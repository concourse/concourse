package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"unicode/utf8"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
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
	build.delegate.Start(logger.Session("start"))

	step := exec.OnComplete(
		exec.Compose(
			build.aggregateInputsStep(logger.Session("inputs")),
			exec.Compose(
				build.executeStep(logger.Session("execute")),
				build.aggregateOutputsStep(logger.Session("outputs")),
			),
		),
		build.delegate.Finish(logger.Session("finish")),
	)

	source := step.Using(&exec.NoopArtifactSource{})

	process := ifrit.Background(source)

	exited := process.Wait()

	for {
		select {
		case <-exited:
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
		origin := event.Origin{
			Type: event.OriginTypeInput,
			Name: input.Name,
		}

		ioConfig := exec.IOConfig{
			Stdout: build.ioWriter(origin),
			Stderr: build.ioWriter(origin),
		}

		resourceConfig := atc.ResourceConfig{
			Name:   input.Resource,
			Type:   input.Type,
			Source: input.Source,
		}

		inputs[input.Name] = exec.OnComplete(
			build.factory.Get(build.inputSessionID(input.Name), ioConfig, resourceConfig, input.Params, input.Version),
			build.delegate.InputCompleted(logger.Session(input.Name), input),
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

	configSource = initializeEmitter{
		buildID:           build.buildID,
		db:                build.db,
		BuildConfigSource: configSource,
	}

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

	return exec.OnComplete(
		build.factory.Execute(build.executeSessionID(), ioConfig, exec.Privileged(plan.Privileged), configSource),
		build.delegate.ExecutionCompleted(logger),
	)
}

func (build *execBuild) aggregateOutputsStep(logger lager.Logger) exec.Step {
	plan := build.metadata.Plan

	outputs := exec.Aggregate{}

	for _, output := range plan.Outputs {
		origin := event.Origin{
			Type: event.OriginTypeOutput,
			Name: output.Name,
		}

		ioConfig := exec.IOConfig{
			Stdout: build.ioWriter(origin),
			Stderr: build.ioWriter(origin),
		}

		resourceConfig := atc.ResourceConfig{
			Name:   output.Name,
			Type:   output.Type,
			Source: output.Source,
		}

		step := exec.OnComplete(
			build.factory.Put(build.outputSessionID(output.Name), ioConfig, resourceConfig, output.Params),
			build.delegate.OutputCompleted(logger.Session(output.Name), output),
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

func (build *execBuild) ioWriter(origin event.Origin) io.Writer {
	return &dbEventWriter{
		buildID: build.buildID,
		db:      build.db,
		origin:  origin,
	}
}

type initializeEmitter struct {
	buildID int
	db      EngineDB

	exec.BuildConfigSource
}

func (emitter initializeEmitter) FetchConfig(source exec.ArtifactSource) (atc.BuildConfig, error) {
	config, err := emitter.BuildConfigSource.FetchConfig(source)
	if err != nil {
		return atc.BuildConfig{}, err
	}

	emitter.db.SaveBuildEvent(emitter.buildID, event.Initialize{
		BuildConfig: config,
	})

	return config, nil
}

type dbEventWriter struct {
	buildID int
	db      EngineDB

	origin event.Origin

	dangling []byte
}

func (writer *dbEventWriter) Write(data []byte) (int, error) {
	text := append(writer.dangling, data...)

	checkEncoding, _ := utf8.DecodeLastRune(text)
	if checkEncoding == utf8.RuneError {
		writer.dangling = text
		return len(data), nil
	}

	writer.dangling = nil

	writer.db.SaveBuildEvent(writer.buildID, event.Log{
		Payload: string(text),
		Origin:  writer.origin,
	})

	return len(data), nil
}
