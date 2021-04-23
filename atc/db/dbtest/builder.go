package dbtest

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	uuid "github.com/nu7hatch/gouuid"
)

const BaseResourceType = "global-base-type"
const UniqueBaseResourceType = "unique-base-type"

type JobInputs []JobInput

type JobInput struct {
	Name            string
	Version         atc.Version
	PassedBuilds    []db.Build
	FirstOccurrence bool

	ResolveError string
}

func (inputs JobInputs) Lookup(name string) (JobInput, bool) {
	for _, i := range inputs {
		if i.Name == name {
			return i, true
		}
	}

	return JobInput{}, false
}

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
		var w db.Worker
		var err error
		if worker.Team != "" {
			team, found, err := builder.TeamFactory.FindTeam(worker.Team)
			if err != nil {
				return err
			}

			if !found {
				return fmt.Errorf("team does not exist: %s", worker.Team)
			}

			w, err = team.SaveWorker(worker, 0)
			if err != nil {
				return err
			}
		} else {
			w, err = builder.WorkerFactory.SaveWorker(worker, 0)
		}
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

		var from db.ConfigVersion
		if scenario.Pipeline != nil {
			from = scenario.Pipeline.ConfigVersion()
		}

		p, _, err := scenario.Team.SavePipeline(atc.PipelineRef{Name: "some-pipeline"}, config, from, false)
		if err != nil {
			return err
		}

		scenario.Pipeline = p
		return nil
	}
}

func (builder Builder) WithBaseWorker() SetupFunc {
	return builder.WithWorker(atc.Worker{
		Name: unique("worker"),

		GardenAddr:      unique("garden-addr"),
		BaggageclaimURL: unique("baggageclaim-url"),

		ResourceTypes: []atc.WorkerResourceType{
			{
				Type:    BaseResourceType,
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
	})
}

func (builder Builder) WithResourceVersions(resourceName string, versions ...atc.Version) SetupFunc {
	return func(scenario *Scenario) error {
		if scenario.Pipeline == nil {
			err := builder.WithPipeline(atc.Config{
				Resources: atc.ResourceConfigs{
					{
						Name:   resourceName,
						Type:   BaseResourceType,
						Source: atc.Source{"some": "source"},
					},
				},
			})(scenario)
			if err != nil {
				return fmt.Errorf("bootstrap pipeline: %w", err)
			}
		}

		if len(scenario.Workers) == 0 {
			err := builder.WithBaseWorker()(scenario)
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

		err = scope.SaveVersions(scenario.SpanContext, versions)
		if err != nil {
			return fmt.Errorf("save versions: %w", err)
		}

		_, err = scope.UpdateLastCheckEndTime(true)
		if err != nil {
			return fmt.Errorf("update last check end time: %w", err)
		}

		err = resource.SetResourceConfigScope(scope)
		if err != nil {
			return fmt.Errorf("set resource scope: %w", err)
		}

		return nil
	}
}

func (builder Builder) WithResourceTypeVersions(resourceTypeName string, versions ...atc.Version) SetupFunc {
	return func(scenario *Scenario) error {
		if scenario.Pipeline == nil {
			err := builder.WithPipeline(atc.Config{
				ResourceTypes: atc.ResourceTypes{
					{
						Name:   resourceTypeName,
						Type:   BaseResourceType,
						Source: atc.Source{"some": "source"},
					},
				},
			})(scenario)
			if err != nil {
				return fmt.Errorf("bootstrap pipeline: %w", err)
			}
		}

		if len(scenario.Workers) == 0 {
			err := builder.WithBaseWorker()(scenario)
			if err != nil {
				return fmt.Errorf("bootstrap workers: %w", err)
			}
		}

		resourceType, found, err := scenario.Pipeline.ResourceType(resourceTypeName)
		if err != nil {
			return err
		}

		if !found {
			return fmt.Errorf("resource type '%s' not configured in pipeline", resourceTypeName)
		}

		resourceTypes, err := scenario.Pipeline.ResourceTypes()
		if err != nil {
			return fmt.Errorf("get pipeline resource types: %w", err)
		}

		resourceConfig, err := builder.ResourceConfigFactory.FindOrCreateResourceConfig(
			resourceType.Type(),
			resourceType.Source(),
			resourceTypes.Filter(resourceType).Deserialize(),
		)
		if err != nil {
			return fmt.Errorf("find or create resource config: %w", err)
		}

		scope, err := resourceConfig.FindOrCreateScope(nil)
		if err != nil {
			return fmt.Errorf("find or create scope: %w", err)
		}

		err = scope.SaveVersions(db.SpanContext{}, versions)
		if err != nil {
			return fmt.Errorf("save versions: %w", err)
		}

		resourceType.SetResourceConfigScope(scope)
		if err != nil {
			return fmt.Errorf("set resource scope: %w", err)
		}

		return nil
	}
}

func (builder Builder) WithPendingJobBuild(assign *db.Build, jobName string) SetupFunc {
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

		build, err := job.CreateBuild("some-user")
		if err != nil {
			return fmt.Errorf("create build: %w", err)
		}

		*assign = build

		return nil
	}
}

func (builder Builder) WithNextInputMapping(jobName string, inputs JobInputs) SetupFunc {
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

		jobInputs, err := job.AlgorithmInputs()
		if err != nil {
			return fmt.Errorf("get job inputs: %w", err)
		}

		var hasErrors bool
		mapping := db.InputMapping{}
		for _, input := range jobInputs {
			i, found := inputs.Lookup(input.Name)
			if !found {
				return fmt.Errorf("no version specified for input '%s'", input.Name)
			}

			buildIDs := []int{}
			for _, build := range i.PassedBuilds {
				buildIDs = append(buildIDs, build.ID())
			}

			mapping[input.Name] = db.InputResult{
				Input: &db.AlgorithmInput{
					AlgorithmVersion: db.AlgorithmVersion{
						Version:    db.ResourceVersion(md5Version(i.Version)),
						ResourceID: input.ResourceID,
					},
					FirstOccurrence: i.FirstOccurrence,
				},
				PassedBuildIDs: buildIDs,
				ResolveError:   db.ResolutionFailure(i.ResolveError),
			}

			if i.ResolveError != "" {
				hasErrors = true
			}
		}

		err = job.SaveNextInputMapping(mapping, !hasErrors)
		if err != nil {
			return fmt.Errorf("save job input mapping: %w", err)
		}

		return nil
	}
}

func (builder Builder) WithJobBuild(assign *db.Build, jobName string, inputs JobInputs, outputs JobOutputs) SetupFunc {
	return func(scenario *Scenario) error {
		var build db.Build
		scenario.Run(
			builder.WithPendingJobBuild(&build, jobName),
			builder.WithNextInputMapping(jobName, inputs),
		)

		_, inputsReady, err := build.AdoptInputsAndPipes()
		if err != nil {
			return fmt.Errorf("adopt inputs and pipes: %w", err)
		}

		if !inputsReady {
			return fmt.Errorf("inputs not available?")
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

		jobOutputs, err := job.Outputs()
		if err != nil {
			return fmt.Errorf("get job outputs: %w", err)
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

		found, err = build.Reload()
		if err != nil {
			return fmt.Errorf("reload build: %w", err)
		}

		if !found {
			return fmt.Errorf("build disappeared")
		}

		*assign = build

		return nil
	}
}

func (builder Builder) WithCheckContainer(resourceName string, workerName string) SetupFunc {
	return func(scenario *Scenario) error {
		if scenario.Pipeline == nil {
			return fmt.Errorf("no pipeline set in scenario")
		}

		if len(scenario.Workers) == 0 {
			return fmt.Errorf("no workers set in scenario")
		}

		resource, found, err := scenario.Pipeline.Resource(resourceName)
		if err != nil {
			return err
		}

		if !found {
			return fmt.Errorf("resource '%s' not configured in pipeline", resourceName)
		}

		rc, found, err := builder.ResourceConfigFactory.FindResourceConfigByID(resource.ResourceConfigID())
		if err != nil {
			return err
		}

		if !found {
			return fmt.Errorf("resource config '%d' not found", rc.ID())
		}

		owner := db.NewResourceConfigCheckSessionContainerOwner(
			rc.ID(),
			rc.OriginBaseResourceType().ID,
			db.ContainerOwnerExpiries{
				Min: 5 * time.Minute,
				Max: 5 * time.Minute,
			},
		)

		worker, found, err := builder.WorkerFactory.GetWorker(workerName)
		if err != nil {
			return err
		}

		if !found {
			return fmt.Errorf("worker '%d' not set in the scenario", rc.ID())
		}

		containerMetadata := db.ContainerMetadata{
			Type: "check",
		}

		_, err = worker.CreateContainer(owner, containerMetadata)
		if err != nil {
			return err
		}

		return nil
	}
}

func (builder Builder) WithSpanContext(spanContext db.SpanContext) SetupFunc {
	return func(scenario *Scenario) error {
		scenario.SpanContext = spanContext
		return nil
	}
}

func (builder Builder) WithVersionMetadata(resourceName string, version atc.Version, metadata db.ResourceConfigMetadataFields) SetupFunc {
	return func(scenario *Scenario) error {
		resource, found, err := scenario.Pipeline.Resource(resourceName)
		if err != nil {
			return err
		}

		if !found {
			return fmt.Errorf("resource '%s' not configured in pipeline", resourceName)
		}

		_, err = resource.UpdateMetadata(version, metadata)
		if err != nil {
			return err
		}

		return nil
	}
}

func (builder Builder) WithPinnedVersion(resourceName string, pinnedVersion atc.Version) SetupFunc {
	return func(scenario *Scenario) error {
		resource, found, err := scenario.Pipeline.Resource(resourceName)
		if err != nil {
			return err
		}

		if !found {
			return fmt.Errorf("resource '%s' not configured in pipeline", resourceName)
		}

		version, found, err := resource.FindVersion(pinnedVersion)
		if err != nil {
			return err
		}

		if !found {
			scenario.Run(builder.WithResourceVersions(resourceName, pinnedVersion))

			reloaded, err := resource.Reload()
			if err != nil {
				return err
			}

			if !reloaded {
				return fmt.Errorf("resource '%s' not reloaded", resourceName)
			}

			version, found, err = resource.FindVersion(pinnedVersion)
			if err != nil {
				return err
			}

			if !found {
				return fmt.Errorf("version '%v' not able to be saved", pinnedVersion)
			}
		}

		_, err = resource.PinVersion(version.ID())
		if err != nil {
			return err
		}

		if !found {
			return fmt.Errorf("version '%v' not pinned", version)
		}

		return nil
	}
}

func (builder Builder) WithDisabledVersion(resourceName string, disabledVersion atc.Version) SetupFunc {
	return func(scenario *Scenario) error {
		resource, found, err := scenario.Pipeline.Resource(resourceName)
		if err != nil {
			return err
		}

		if !found {
			return fmt.Errorf("resource '%s' not configured in pipeline", resourceName)
		}

		version, found, err := resource.FindVersion(disabledVersion)
		if err != nil {
			return err
		}

		if !found {
			scenario.Run(builder.WithResourceVersions(resourceName, disabledVersion))

			reloaded, err := resource.Reload()
			if err != nil {
				return err
			}

			if !reloaded {
				return fmt.Errorf("resource '%s' not reloaded", resourceName)
			}

			version, found, err = resource.FindVersion(disabledVersion)
			if err != nil {
				return err
			}

			if !found {
				return fmt.Errorf("version '%v' not able to be saved", disabledVersion)
			}
		}

		err = resource.DisableVersion(version.ID())
		if err != nil {
			return err
		}

		return nil
	}
}

func (builder Builder) WithEnabledVersion(resourceName string, enabledVersion atc.Version) SetupFunc {
	return func(scenario *Scenario) error {
		resource, found, err := scenario.Pipeline.Resource(resourceName)
		if err != nil {
			return err
		}

		if !found {
			return fmt.Errorf("resource '%s' not configured in pipeline", resourceName)
		}

		version, found, err := resource.FindVersion(enabledVersion)
		if err != nil {
			return err
		}

		if found {
			err = resource.EnableVersion(version.ID())
			if err != nil {
				return err
			}
		} else {
			scenario.Run(builder.WithResourceVersions(resourceName, enabledVersion))
		}

		return nil
	}
}

func (builder Builder) WithBaseResourceType(dbConn db.Conn, resourceTypeName string) SetupFunc {
	return func(scenario *Scenario) error {
		setupTx, err := dbConn.Begin()
		if err != nil {
			return err
		}

		brt := db.BaseResourceType{
			Name: resourceTypeName,
		}

		_, err = brt.FindOrCreate(setupTx, false)
		if err != nil {
			return err
		}

		return setupTx.Commit()
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
