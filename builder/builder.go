package builder

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	TurbineBuilds "github.com/concourse/turbine/api/builds"
	TurbineRoutes "github.com/concourse/turbine/routes"
	"github.com/tedsuo/router"

	WinstonRoutes "github.com/concourse/atc/api/routes"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
)

var ErrBadResponse = errors.New("bad response from turbine")

type Builder interface {
	Create(config.Job) (builds.Build, error)
	Attempt(config.Job, config.Resource, builds.Version) (builds.Build, error)
	Start(config.Job, builds.Build, map[string]builds.Version) (builds.Build, error)
}

type builder struct {
	db        db.DB
	resources config.Resources

	turbine *router.RequestGenerator
	atc     *router.RequestGenerator

	httpClient *http.Client
}

func NewBuilder(
	db db.DB,
	resources config.Resources,
	turbine *router.RequestGenerator,
	atc *router.RequestGenerator,
) Builder {
	return &builder{
		db:        db,
		resources: resources,

		turbine: turbine,
		atc:     atc,

		httpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 5 * time.Minute,
			},
		},
	}
}

func (builder *builder) Create(job config.Job) (builds.Build, error) {
	return builder.db.CreateBuild(job.Name)
}

func (builder *builder) Attempt(job config.Job, resource config.Resource, version builds.Version) (builds.Build, error) {
	hasOutput := false
	for _, out := range job.Outputs {
		if out.Resource == resource.Name {
			hasOutput = true
		}
	}

	return builder.db.AttemptBuild(job.Name, resource.Name, version, hasOutput)
}

func (builder *builder) Start(job config.Job, build builds.Build, versionOverrides map[string]builds.Version) (builds.Build, error) {
	versions, err := builder.computeVersions(job, versionOverrides)
	if err != nil {
		return builds.Build{}, err
	}

	inputs, err := builder.computeInputs(job, versions)
	if err != nil {
		return builds.Build{}, err
	}

	outputs, err := builder.computeOutputs(job)
	if err != nil {
		return builds.Build{}, err
	}

	scheduled, err := builder.db.ScheduleBuild(job.Name, build.ID, job.Serial)
	if err != nil {
		return builds.Build{}, err
	}

	if !scheduled {
		return builder.db.GetBuild(job.Name, build.ID)
	}

	complete, err := builder.atc.RequestForHandler(
		WinstonRoutes.UpdateBuild,
		router.Params{
			"job":   job.Name,
			"build": fmt.Sprintf("%d", build.ID),
		},
		nil,
	)
	if err != nil {
		panic(err)
	}

	logs, err := builder.atc.RequestForHandler(
		WinstonRoutes.LogInput,
		router.Params{
			"job":   job.Name,
			"build": fmt.Sprintf("%d", build.ID),
		},
		nil,
	)
	if err != nil {
		panic(err)
	}

	logs.URL.Scheme = "ws"

	buildConfig := TurbineBuilds.Config{
		Inputs:  inputs,
		Outputs: outputs,
	}.Merge(job.BuildConfig)

	turbineBuild := TurbineBuilds.Build{
		Config: buildConfig,

		Privileged: job.Privileged,

		Callback: complete.URL.String(),
		LogsURL:  logs.URL.String(),
	}

	req := new(bytes.Buffer)

	err = json.NewEncoder(req).Encode(turbineBuild)
	if err != nil {
		return builds.Build{}, err
	}

	execute, err := builder.turbine.RequestForHandler(
		TurbineRoutes.ExecuteBuild,
		nil,
		req,
	)
	if err != nil {
		return builds.Build{}, err
	}

	execute.Header.Set("Content-Type", "application/json")

	resp, err := builder.httpClient.Do(execute)
	if err != nil {
		return builds.Build{}, err
	}

	// TODO test bad response code
	if resp.StatusCode != http.StatusCreated {
		return builds.Build{}, ErrBadResponse
	}

	var startedBuild TurbineBuilds.Build
	err = json.NewDecoder(resp.Body).Decode(&startedBuild)
	if err != nil {
		return builds.Build{}, err
	}

	resp.Body.Close()

	started, err := builder.db.StartBuild(job.Name, build.ID, startedBuild.AbortURL)
	if !started {
		builder.httpClient.Post(startedBuild.AbortURL, "", nil)
	}

	return builder.db.GetBuild(job.Name, build.ID)
}

func (builder *builder) computeVersions(job config.Job, versionOverrides map[string]builds.Version) (map[string]builds.Version, error) {
	versions := map[string]builds.Version{}

	for _, input := range job.Inputs {
		version, found := versionOverrides[input.Resource]
		if found {
			versions[input.Resource] = version
		}

		if input.Passed == nil {
			continue
		}

		commonVersions, err := builder.db.GetCommonOutputs(input.Passed, input.Resource)
		if err != nil {
			return nil, err
		}

		if len(commonVersions) == 0 {
			return nil, fmt.Errorf("unsatisfied input: %s; depends on %v\n", input.Resource, input.Passed)
		}

		versions[input.Resource] = commonVersions[len(commonVersions)-1]
	}

	return versions, nil
}

func (builder *builder) computeInputs(job config.Job, versions map[string]builds.Version) ([]TurbineBuilds.Input, error) {
	turbineInputs := make([]TurbineBuilds.Input, len(job.Inputs))
	for i, input := range job.Inputs {
		resource, found := builder.resources.Lookup(input.Resource)
		if !found {
			return nil, fmt.Errorf("unknown resource: %s", input.Resource)
		}

		turbineInputs[i] = builder.inputFor(job, resource, versions[input.Resource])
	}

	return turbineInputs, nil
}

func (builder *builder) inputFor(job config.Job, resource config.Resource, version builds.Version) TurbineBuilds.Input {
	turbineInput := TurbineBuilds.Input{
		Name:            resource.Name,
		Type:            resource.Type,
		Source:          TurbineBuilds.Source(resource.Source),
		Version:         TurbineBuilds.Version(version),
		DestinationPath: resource.Name,
	}

	if filepath.HasPrefix(job.BuildConfigPath, resource.Name) {
		turbineInput.ConfigPath = job.BuildConfigPath[len(resource.Name)+1:]
	}

	return turbineInput
}

func (builder *builder) computeOutputs(job config.Job) ([]TurbineBuilds.Output, error) {
	turbineOutputs := []TurbineBuilds.Output{}
	for _, output := range job.Outputs {
		resource, found := builder.resources.Lookup(output.Resource)
		if !found {
			return nil, fmt.Errorf("unknown resource: %s", output.Resource)
		}

		turbineOutput := TurbineBuilds.Output{
			Name:       resource.Name,
			Type:       resource.Type,
			Params:     TurbineBuilds.Params(output.Params),
			Source:     TurbineBuilds.Source(resource.Source),
			SourcePath: resource.Name,
		}

		turbineOutputs = append(turbineOutputs, turbineOutput)
	}

	return turbineOutputs, nil
}
