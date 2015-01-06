package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
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
	factory exec.Factory
	db      EngineDB
}

func NewGardenEngine(factory exec.Factory, db EngineDB) Engine {
	return &gardenEngine{
		factory: factory,
		db:      db,
	}
}

func (engine *gardenEngine) Name() string {
	return "garden.v1"
}

func (engine *gardenEngine) CreateBuild(model db.Build, plan atc.BuildPlan) (Build, error) {
	return &gardenBuild{
		buildID: model.ID,
		db:      engine.db,
		factory: engine.factory,
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
		metadata: metadata,

		signals: make(chan os.Signal, 1),
	}, nil
}

type gardenBuild struct {
	buildID int
	db      EngineDB

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
	build.signals <- os.Kill
	return nil
}

type implicitOutput struct {
	plan atc.InputPlan
	info exec.VersionInfo
}

type implicitOutputTracker struct {
	implicitOutputs  map[string]implicitOutput
	implicitOutputsL sync.Mutex
}

func (t *implicitOutputTracker) register(name string, output implicitOutput) {
	t.implicitOutputsL.Lock()
	t.implicitOutputs[name] = output
	t.implicitOutputsL.Unlock()
}

func (t *implicitOutputTracker) unregister(name string) {
	t.implicitOutputsL.Lock()
	delete(t.implicitOutputs, name)
	t.implicitOutputsL.Unlock()
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
func (build *gardenBuild) Resume(lager.Logger) {
	plan := build.metadata.Plan

	implicitOutputsT := &implicitOutputTracker{
		implicitOutputs: map[string]implicitOutput{},
	}

	var step exec.Step

	if len(plan.Inputs) > 0 {
		inputs := exec.Aggregate{}

		for _, input := range plan.Inputs {
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

			plan := input
			inputs[input.Name] = exec.OnComplete(
				build.factory.Get(build.inputSessionID(input.Name), ioConfig, resourceConfig, input.Params, input.Version),
				func(err error, source exec.ArtifactSource) {
					if err != nil {
						build.saveErr(err, origin)
					} else {
						var info exec.VersionInfo
						if source.Result(&info) {
							build.saveInput(plan, info)
						}

						implicitOutputsT.register(plan.Resource, implicitOutput{plan, info})
					}
				},
			)
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

	successReporter := exec.NewSuccessReporter()

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

		configSource = initializeEmitter{
			buildID:           build.buildID,
			db:                build.db,
			BuildConfigSource: configSource,
		}

		step = exec.Compose(step, exec.OnComplete(
			successReporter.Subject(build.factory.Execute(build.executeSessionID(), ioConfig, configSource)),
			func(err error, source exec.ArtifactSource) {
				if err != nil {
					build.saveErr(err, event.Origin{})
				} else {
					var status exec.ExitStatus
					if source.Result(&status) {
						build.saveFinish(status)
					}
				}
			},
		))
	}

	if len(plan.Outputs) > 0 {
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

			plan := output

			outputs[output.Name] = exec.Conditional{
				Conditions: output.On,
				Step: exec.OnComplete(
					build.factory.Put(build.outputSessionID(output.Name), ioConfig, resourceConfig, output.Params),
					func(err error, source exec.ArtifactSource) {
						if err != nil {
							build.saveErr(err, origin)
						} else {
							implicitOutputsT.unregister(plan.Name)

							var info exec.VersionInfo
							if source.Result(&info) {
								build.saveOutput(plan, info)
							}
						}
					},
				),
			}
		}

		step = exec.Compose(step, outputs)
	}

	source := step.Using(&exec.NoopArtifactSource{})

	build.saveStart()

	process := ifrit.Background(source)

	exited := process.Wait()

	var aborted bool

	for {
		select {
		case err := <-exited:
			if aborted {
				build.saveStatus(atc.StatusAborted)
			} else if err != nil {
				build.saveStatus(atc.StatusErrored)
			} else if successReporter.Successful() {
				build.saveStatus(atc.StatusSucceeded)

				for _, o := range implicitOutputsT.implicitOutputs {
					build.saveImplicitOutput(o.plan, o.info)
				}
			} else {
				build.saveStatus(atc.StatusFailed)
			}

			return

		case sig := <-build.signals:
			process.Signal(sig)

			if sig == os.Kill {
				aborted = true
			}
		}
	}
}

func (build *gardenBuild) Hijack(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	return build.factory.Hijack(build.executeSessionID(), spec, io)
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

func (build *gardenBuild) saveStart() {
	// TODO handle errs

	time := time.Now()

	build.db.SaveBuildStartTime(build.buildID, time)

	build.db.SaveBuildStatus(build.buildID, db.StatusStarted)

	build.db.SaveBuildEvent(build.buildID, event.Start{
		Time: time.Unix(),
	})

	build.db.SaveBuildEvent(build.buildID, event.Status{
		Status: atc.StatusStarted,
		Time:   time.Unix(),
	})
}

func (build *gardenBuild) saveFinish(status exec.ExitStatus) {
	build.db.SaveBuildEvent(build.buildID, event.Finish{
		ExitStatus: int(status),
		Time:       time.Now().Unix(),
	})
}

func (build *gardenBuild) saveStatus(status atc.BuildStatus) {
	// TODO handle errs

	time := time.Now()

	build.db.SaveBuildEndTime(build.buildID, time)

	build.db.SaveBuildStatus(build.buildID, db.Status(status))

	build.db.SaveBuildEvent(build.buildID, event.Status{
		Status: status,
		Time:   time.Unix(),
	})

	build.db.CompleteBuild(build.buildID)
}

func (build *gardenBuild) saveErr(err error, origin event.Origin) {
	// TODO handle errs

	build.db.SaveBuildEvent(build.buildID, event.Error{
		Message: err.Error(),
		Origin:  origin,
	})
}

func (build *gardenBuild) saveInput(plan atc.InputPlan, info exec.VersionInfo) {
	ev := event.Input{
		Plan:            plan,
		FetchedVersion:  info.Version,
		FetchedMetadata: info.Metadata,
	}

	build.db.SaveBuildEvent(build.buildID, ev)

	build.db.SaveBuildInput(build.buildID, db.BuildInput{
		Name:              plan.Name,
		VersionedResource: vrFromInput(ev),
	})
}

func (build *gardenBuild) saveOutput(plan atc.OutputPlan, info exec.VersionInfo) {
	ev := event.Output{
		Plan:            plan,
		CreatedVersion:  info.Version,
		CreatedMetadata: info.Metadata,
	}

	build.db.SaveBuildEvent(build.buildID, ev)

	build.db.SaveBuildOutput(build.buildID, vrFromOutput(ev))
}

func (build *gardenBuild) saveImplicitOutput(plan atc.InputPlan, info exec.VersionInfo) {
	metadata := make([]db.MetadataField, len(info.Metadata))
	for i, md := range info.Metadata {
		metadata[i] = db.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	build.db.SaveBuildOutput(build.buildID, db.VersionedResource{
		Resource: plan.Resource,
		Type:     plan.Type,
		Source:   db.Source(plan.Source),
		Version:  db.Version(info.Version),
		Metadata: metadata,
	})
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

	// TODO handle err
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

	// TODO handle err
	writer.db.SaveBuildEvent(writer.buildID, event.Log{
		Payload: string(text),
		Origin:  writer.origin,
	})

	return len(data), nil
}
