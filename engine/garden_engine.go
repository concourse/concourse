package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"unicode/utf8"

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
}

type gardenEngine struct {
	factory         exec.Factory
	delegateFactory BuildDelegateFactory
	db              EngineDB
}

func NewGardenEngine(factory exec.Factory, delegateFactory BuildDelegateFactory, db EngineDB) Engine {
	return &gardenEngine{
		factory:         factory,
		delegateFactory: delegateFactory,
		db:              db,
	}
}

func (engine *gardenEngine) Name() string {
	return "garden.v1"
}

func (engine *gardenEngine) CreateBuild(model db.Build, plan atc.BuildPlan) (Build, error) {
	return &gardenBuild{
		buildID:  model.ID,
		db:       engine.db,
		factory:  engine.factory,
		delegate: engine.delegateFactory.Delegate(model.ID),
		metadata: GardenMetadata{
			Plan: plan,
		},

		signals: make(chan os.Signal, 1),
	}, nil
}

func (engine *gardenEngine) LookupBuild(model db.Build) (Build, error) {
	var metadata GardenMetadata
	err := json.Unmarshal([]byte(model.EngineMetadata), &metadata)
	if err != nil {
		return nil, err
	}

	return &gardenBuild{
		buildID:  model.ID,
		db:       engine.db,
		factory:  engine.factory,
		delegate: engine.delegateFactory.Delegate(model.ID),
		metadata: metadata,

		signals: make(chan os.Signal, 1),
	}, nil
}

type gardenBuild struct {
	buildID int
	db      EngineDB

	factory  exec.Factory
	delegate BuildDelegate

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
	build.signals <- os.Kill
	return nil
}

func (build *gardenBuild) Resume(lager.Logger) {
	build.delegate.Start()

	step := exec.OnComplete(
		exec.Compose(
			build.aggregateInputsStep(),
			exec.Compose(
				build.executeStep(),
				build.aggregateOutputsStep(),
			),
		),
		build.delegate.Finish(),
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
				build.delegate.Aborted()
			}
		}
	}
}

func (build *gardenBuild) Hijack(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	return build.factory.Hijack(build.executeSessionID(), spec, io)
}

func (build *gardenBuild) aggregateInputsStep() exec.Step {
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
			build.delegate.InputCompleted(input),
		)
	}

	return inputs
}

func (build *gardenBuild) executeStep() exec.Step {
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
		build.factory.Execute(build.executeSessionID(), ioConfig, configSource),
		build.delegate.ExecutionCompleted(),
	)
}

func (build *gardenBuild) aggregateOutputsStep() exec.Step {
	outputs := exec.Aggregate{}

	for _, output := range build.metadata.Plan.Outputs {
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

		outputs[output.Name] = exec.Conditional{
			Conditions: output.On,
			Step: exec.OnComplete(
				build.factory.Put(build.outputSessionID(output.Name), ioConfig, resourceConfig, output.Params),
				build.delegate.OutputCompleted(output),
			),
		}
	}

	return outputs
}

func (build *gardenBuild) executeSessionID() exec.SessionID {
	return exec.SessionID(fmt.Sprintf("build-%d-execute", build.buildID))
}

func (build *gardenBuild) inputSessionID(inputName string) exec.SessionID {
	return exec.SessionID(fmt.Sprintf("build-%d-input-%s", build.buildID, inputName))
}

func (build *gardenBuild) outputSessionID(outputName string) exec.SessionID {
	return exec.SessionID(fmt.Sprintf("build-%d-output-%s", build.buildID, outputName))
}

func (build *gardenBuild) ioWriter(origin event.Origin) io.Writer {
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
