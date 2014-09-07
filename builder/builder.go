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
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/builds"
	CallbacksRoutes "github.com/concourse/atc/callbacks/routes"
	"github.com/concourse/atc/config"
)

var ErrBadResponse = errors.New("bad response from turbine")

type BuilderDB interface {
	ScheduleBuild(job string, build string, serial bool) (bool, error)
	StartBuild(job string, build string, abortURL string) (bool, error)
}

type Builder interface {
	Build(builds.Build, config.Job, builds.VersionedResources) error
}

type builder struct {
	db        BuilderDB
	resources config.Resources

	turbine *rata.RequestGenerator
	atc     *rata.RequestGenerator

	httpClient *http.Client
}

func NewBuilder(db BuilderDB, resources config.Resources, turbine *rata.RequestGenerator, atc *rata.RequestGenerator) Builder {
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

func (builder *builder) Build(build builds.Build, job config.Job, versions builds.VersionedResources) error {
	scheduled, err := builder.db.ScheduleBuild(job.Name, build.Name, job.Serial)
	if err != nil {
		return err
	}

	if !scheduled {
		return nil
	}

	inputs, err := builder.computeInputs(job, versions)
	if err != nil {
		return err
	}

	outputs, err := builder.computeOutputs(job)
	if err != nil {
		return err
	}

	updateBuild, err := builder.atc.CreateRequest(
		CallbacksRoutes.UpdateBuild,
		rata.Params{
			"build": fmt.Sprintf("%d", build.ID),
		},
		nil,
	)
	if err != nil {
		panic(err)
	}

	recordEvents, err := builder.atc.CreateRequest(
		CallbacksRoutes.RecordEvents,
		rata.Params{
			"build": fmt.Sprintf("%d", build.ID),
		},
		nil,
	)
	if err != nil {
		panic(err)
	}

	recordEvents.URL.Scheme = "ws"

	turbineBuild := TurbineBuilds.Build{
		Config: job.BuildConfig,

		Inputs:  inputs,
		Outputs: outputs,

		Privileged: job.Privileged,

		StatusCallback: updateBuild.URL.String(),
		EventsCallback: recordEvents.URL.String(),
	}

	req := new(bytes.Buffer)

	err = json.NewEncoder(req).Encode(turbineBuild)
	if err != nil {
		return err
	}

	execute, err := builder.turbine.CreateRequest(
		TurbineRoutes.ExecuteBuild,
		nil,
		req,
	)
	if err != nil {
		return err
	}

	execute.Header.Set("Content-Type", "application/json")

	resp, err := builder.httpClient.Do(execute)
	if err != nil {
		return err
	}

	// TODO test bad response code
	if resp.StatusCode != http.StatusCreated {
		return ErrBadResponse
	}

	var startedBuild TurbineBuilds.Build
	err = json.NewDecoder(resp.Body).Decode(&startedBuild)
	if err != nil {
		return err
	}

	resp.Body.Close()

	started, err := builder.db.StartBuild(job.Name, build.Name, startedBuild.AbortURL)
	if err != nil {
		return err
	}

	if !started {
		builder.httpClient.Post(startedBuild.AbortURL, "", nil)
	}

	return nil
}

func (builder *builder) computeInputs(job config.Job, inputs builds.VersionedResources) ([]TurbineBuilds.Input, error) {
	turbineInputs := make([]TurbineBuilds.Input, len(job.Inputs))
	for i, input := range job.Inputs {
		resource, found := builder.resources.Lookup(input.Resource)
		if !found {
			return nil, fmt.Errorf("unknown resource: %s", input.Resource)
		}

		vr, found := inputs.Lookup(input.Resource)
		if !found {
			vr = builds.VersionedResource{
				Name:   resource.Name,
				Type:   resource.Type,
				Source: resource.Source,
			}
		}

		turbineInputs[i] = builder.inputFor(job, vr, input.Params)
	}

	return turbineInputs, nil
}

func (builder *builder) inputFor(job config.Job, vr builds.VersionedResource, params config.Params) TurbineBuilds.Input {
	turbineInput := TurbineBuilds.Input{
		Name:    vr.Name,
		Type:    vr.Type,
		Source:  TurbineBuilds.Source(vr.Source),
		Version: TurbineBuilds.Version(vr.Version),
		Params:  TurbineBuilds.Params(params),
	}

	if filepath.HasPrefix(job.BuildConfigPath, vr.Name) {
		turbineInput.ConfigPath = job.BuildConfigPath[len(vr.Name)+1:]
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

		conditions := []TurbineBuilds.OutputCondition{}

		// if not specified, assume [success]
		//
		// note that this check is for nil, not len(output.On) == 0
		if output.On == nil {
			conditions = append(conditions, TurbineBuilds.OutputConditionSuccess)
		} else {
			for _, cond := range output.On {
				conditions = append(conditions, TurbineBuilds.OutputCondition(cond))
			}
		}

		turbineOutput := TurbineBuilds.Output{
			Name:   resource.Name,
			Type:   resource.Type,
			On:     conditions,
			Params: TurbineBuilds.Params(output.Params),
			Source: TurbineBuilds.Source(resource.Source),
		}

		turbineOutputs = append(turbineOutputs, turbineOutput)
	}

	return turbineOutputs, nil
}
