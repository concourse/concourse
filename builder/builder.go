package builder

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/tedsuo/router"
	ProleBuilds "github.com/winston-ci/prole/api/builds"
	ProleRoutes "github.com/winston-ci/prole/routes"

	WinstonRoutes "github.com/winston-ci/winston/api/routes"
	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/db"
)

var ErrBadResponse = errors.New("bad response from prole")

type Builder interface {
	Create(config.Job) (builds.Build, error)
	Start(config.Job, builds.Build, map[string]builds.Version) (builds.Build, error)
}

type builder struct {
	db        db.DB
	resources config.Resources

	prole   *router.RequestGenerator
	winston *router.RequestGenerator

	httpClient *http.Client
}

func NewBuilder(
	db db.DB,
	resources config.Resources,
	prole *router.RequestGenerator,
	winston *router.RequestGenerator,
) Builder {
	return &builder{
		db:        db,
		resources: resources,

		prole:   prole,
		winston: winston,

		httpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 5 * time.Minute,
			},
		},
	}
}

func (builder *builder) Create(job config.Job) (builds.Build, error) {
	log.Println("creating build")
	return builder.db.CreateBuild(job.Name)
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

	complete, err := builder.winston.RequestForHandler(
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

	log.Println("completion callback:", complete.URL)

	logs, err := builder.winston.RequestForHandler(
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

	log.Println("logs callback:", logs.URL)

	logs.URL.Scheme = "ws"

	proleBuild := ProleBuilds.Build{
		Privileged: job.Privileged,

		Inputs:  inputs,
		Outputs: outputs,

		Callback: complete.URL.String(),
		LogsURL:  logs.URL.String(),
	}

	log.Printf("creating build: %#v\n", proleBuild)

	req := new(bytes.Buffer)

	err = json.NewEncoder(req).Encode(proleBuild)
	if err != nil {
		return builds.Build{}, err
	}

	execute, err := builder.prole.RequestForHandler(
		ProleRoutes.ExecuteBuild,
		nil,
		req,
	)
	if err != nil {
		return builds.Build{}, err
	}

	execute.Header.Set("Content-Type", "application/json")

	resp, err := builder.httpClient.Do(execute)
	if err != nil {
		log.Println("prole request failed:", err)
		return builds.Build{}, err
	}

	// TODO test bad response code
	if resp.StatusCode != http.StatusCreated {
		log.Println("bad prole response:", resp)
		return builds.Build{}, ErrBadResponse
	}

	var startedBuild ProleBuilds.Build
	err = json.NewDecoder(resp.Body).Decode(&startedBuild)
	if err != nil {
		log.Println("bad prole response (expecting build):", err)
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

func (builder *builder) computeInputs(job config.Job, versions map[string]builds.Version) ([]ProleBuilds.Input, error) {
	proleInputs := []ProleBuilds.Input{}

	added := map[string]bool{}
	for _, input := range job.Inputs {
		resource, found := builder.resources.Lookup(input.Resource)
		if !found {
			return nil, fmt.Errorf("unknown resource: %s", input.Resource)
		}

		proleInputs = append(proleInputs, builder.inputFor(job, resource, versions[input.Resource]))

		added[input.Resource] = true
	}

	for _, output := range job.Outputs {
		if added[output.Resource] {
			continue
		}

		resource, found := builder.resources.Lookup(output.Resource)
		if !found {
			return nil, fmt.Errorf("unknown resource: %s", output.Resource)
		}

		proleInputs = append(proleInputs, builder.inputFor(job, resource, versions[output.Resource]))
	}

	return proleInputs, nil
}

func (builder *builder) inputFor(job config.Job, resource config.Resource, version builds.Version) ProleBuilds.Input {
	proleInput := ProleBuilds.Input{
		Name:            resource.Name,
		Type:            resource.Type,
		Source:          ProleBuilds.Source(resource.Source),
		Version:         ProleBuilds.Version(version),
		DestinationPath: resource.Name,
	}

	if filepath.HasPrefix(job.BuildConfigPath, resource.Name) {
		proleInput.ConfigPath = job.BuildConfigPath[len(resource.Name)+1:]
	}

	return proleInput
}

func (builder *builder) computeOutputs(job config.Job) ([]ProleBuilds.Output, error) {
	proleOutputs := []ProleBuilds.Output{}
	for _, output := range job.Outputs {
		resource, found := builder.resources.Lookup(output.Resource)
		if !found {
			return nil, fmt.Errorf("unknown resource: %s", output.Resource)
		}

		proleOutput := ProleBuilds.Output{
			Name:       resource.Name,
			Type:       resource.Type,
			Params:     ProleBuilds.Params(output.Params),
			SourcePath: resource.Name,
		}

		proleOutputs = append(proleOutputs, proleOutput)
	}

	return proleOutputs, nil
}
