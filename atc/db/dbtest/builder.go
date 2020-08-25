package dbtest

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	uuid "github.com/nu7hatch/gouuid"
)

const GlobalBaseResourceType = "global-base-type"
const UniqueBaseResourceType = "unique-base-type"

type JobInputs map[string]atc.Version
type JobOutputs map[string]atc.Version

type Builder struct {
	TeamFactory           db.TeamFactory
	WorkerFactory         db.WorkerFactory
	ResourceConfigFactory db.ResourceConfigFactory
}

func NewBuilder(conn db.Conn, lockFactory lock.LockFactory) Builder {
	return Builder{
		TeamFactory:           db.NewTeamFactory(conn, lockFactory),
		WorkerFactory:         db.NewWorkerFactory(conn),
		ResourceConfigFactory: db.NewResourceConfigFactory(conn, lockFactory),
	}
}

func (builder Builder) WithTeam(teamName string) SetupFunc {
	return func(scenario *Scenario) error {
		t, err := builder.TeamFactory.CreateTeam(atc.Team{Name: teamName})
		if err != nil {
			return err
		}

		scenario.Team = t
		return nil
	}
}

func (builder Builder) WithWorker(worker atc.Worker) SetupFunc {
	return func(scenario *Scenario) error {
		w, err := builder.WorkerFactory.SaveWorker(worker, 0)
		if err != nil {
			return err
		}

		scenario.Workers = append(scenario.Workers, w)
		return nil
	}
}

func (builder Builder) WithPipeline(config atc.Config) SetupFunc {
	return func(scenario *Scenario) error {
		if scenario.Team == nil {
			err := builder.WithTeam(unique("team"))(scenario)
			if err != nil {
				return fmt.Errorf("bootstrap team: %w", err)
			}
		}

		p, _, err := scenario.Team.SavePipeline(atc.PipelineRef{Name: "some-pipeline"}, config, 0, false)
		if err != nil {
			return err
		}

		// XXX: set up workers with base resource types?

		scenario.Pipeline = p
		return nil
	}
}

func (builder Builder) WithResourceVersions(resourceName string, versions ...atc.Version) SetupFunc {
	return func(scenario *Scenario) error {
		if scenario.Pipeline == nil {
			err := builder.WithPipeline(atc.Config{
				Resources: atc.ResourceConfigs{
					{
						Name:   resourceName,
						Type:   GlobalBaseResourceType,
						Source: atc.Source{"some": "source"},
					},
				},
			})(scenario)
			if err != nil {
				return fmt.Errorf("bootstrap pipeline: %w", err)
			}
		}

		// bootstrap workers to ensure base resource type exists
		if len(scenario.Workers) == 0 {
			err := builder.WithWorker(atc.Worker{
				Name: unique("worker"),

				GardenAddr:      unique("garden-addr"),
				BaggageclaimURL: unique("baggageclaim-url"),

				ResourceTypes: []atc.WorkerResourceType{
					{
						Type:    GlobalBaseResourceType,
						Image:   "/path/to/global/image",
						Version: "some-global-type-version",
					},
					{
						Type:                 UniqueBaseResourceType,
						Image:                "/path/to/unique/image",
						Version:              "some-unique-type-version",
						UniqueVersionHistory: true,
					},
				},
			})(scenario)
			if err != nil {
				return fmt.Errorf("bootstrap workers: %w", err)
			}
		}

		resource, found, err := scenario.Pipeline.Resource(resourceName)
		if err != nil {
			return err
		}

		if !found {
			return fmt.Errorf("resource '%s' not configured in pipeline", resourceName)
		}

		resourceTypes, err := scenario.Pipeline.ResourceTypes()
		if err != nil {
			return fmt.Errorf("get pipeline resource types: %w", err)
		}

		resourceConfig, err := builder.ResourceConfigFactory.FindOrCreateResourceConfig(
			resource.Type(),
			resource.Source(),
			resourceTypes.Deserialize(),
		)
		if err != nil {
			return fmt.Errorf("find or create resource config: %w", err)
		}

		scope, err := resourceConfig.FindOrCreateScope(resource)
		if err != nil {
			return fmt.Errorf("find or create scope: %w", err)
		}

		err = scope.SaveVersions(db.SpanContext{}, versions)
		if err != nil {
			return fmt.Errorf("save versions: %w", err)
		}

		resource.SetResourceConfigScope(scope)
		if err != nil {
			return fmt.Errorf("set resource scope: %w", err)
		}

		return nil
	}
}

func (builder Builder) WithJobBuild(assign *db.Build, jobName string, inputs JobInputs, outputs JobOutputs) SetupFunc {
	return func(scenario *Scenario) error {
		if scenario.Pipeline == nil {
			return fmt.Errorf("no pipeline set in scenario")
		}

		job, found, err := scenario.Pipeline.Job(jobName)
		if err != nil {
			return err
		}

		if !found {
			return fmt.Errorf("job '%s' not configured in pipeline", jobName)
		}

		resourceTypes, err := scenario.Pipeline.ResourceTypes()
		if err != nil {
			return fmt.Errorf("get pipeline resource types: %w", err)
		}

		jobInputs, err := job.AlgorithmInputs()
		if err != nil {
			return fmt.Errorf("get job inputs: %w", err)
		}

		jobOutputs, err := job.Outputs()
		if err != nil {
			return fmt.Errorf("get job outputs: %w", err)
		}

		mapping := db.InputMapping{}
		for _, input := range jobInputs {
			version, found := inputs[input.Name]
			if !found {
				return fmt.Errorf("no version specified for input '%s'", input.Name)
			}

			mapping[input.Name] = db.InputResult{
				Input: &db.AlgorithmInput{
					AlgorithmVersion: db.AlgorithmVersion{
						Version:    db.ResourceVersion(md5Version(version)),
						ResourceID: input.ResourceID,
					},
				},
			}
		}

		err = job.SaveNextInputMapping(mapping, true)
		if err != nil {
			return fmt.Errorf("save job input mapping: %w", err)
		}

		build, err := job.CreateBuild()
		if err != nil {
			return fmt.Errorf("create job build: %w", err)
		}

		*assign = build

		_, inputsReady, err := build.AdoptInputsAndPipes()
		if err != nil {
			return fmt.Errorf("adopt inputs and pipes: %w", err)
		}

		if !inputsReady {
			return fmt.Errorf("inputs not available?")
		}

		for _, output := range jobOutputs {
			version, found := outputs[output.Name]
			if !found {
				return fmt.Errorf("no version specified for output '%s'", output.Name)
			}

			resource, found, err := scenario.Pipeline.Resource(output.Resource)
			if err != nil {
				return fmt.Errorf("get output resource: %w", err)
			}

			if !found {
				return fmt.Errorf("output '%s' refers to unknown resource '%s'", output.Name, output.Resource)
			}

			err = build.SaveOutput(
				resource.Type(),
				resource.Source(),
				resourceTypes.Deserialize(),
				version,
				nil, // metadata
				output.Name,
				output.Resource,
			)
			if err != nil {
				return fmt.Errorf("save build output: %w", err)
			}
		}

		return nil
	}
}

func unique(kind string) string {
	id, err := uuid.NewV4()
	if err != nil {
		panic(err)
	}

	return kind + "-" + id.String()
}

func md5Version(version atc.Version) string {
	versionJSON, err := json.Marshal(version)
	if err != nil {
		panic(err)
	}

	hasher := md5.New()
	hasher.Write([]byte(versionJSON))
	return hex.EncodeToString(hasher.Sum(nil))
}
