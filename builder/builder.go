package builder

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

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
	Build(config.Job, ...config.Resource) (builds.Build, error)
}

type builder struct {
	db        db.DB
	resources config.Resources

	prole   *router.RequestGenerator
	winston *router.RequestGenerator
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
	}
}

func (builder *builder) Build(job config.Job, resourceOverrides ...config.Resource) (builds.Build, error) {
	log.Println("creating build")

	inputs, err := builder.computeInputs(job, config.Resources(resourceOverrides))
	if err != nil {
		return builds.Build{}, err
	}

	build, err := builder.db.CreateBuild(job.Name)
	if err != nil {
		return builds.Build{}, err
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

		Inputs: inputs,

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

	resp, err := http.DefaultClient.Do(execute)
	if err != nil {
		log.Println("prole request failed:", err)
		return builds.Build{}, err
	}

	// TODO test bad response code
	if resp.StatusCode != http.StatusCreated {
		log.Println("bad prole response:", resp)
		return builds.Build{}, ErrBadResponse
	}

	resp.Body.Close()

	return build, nil
}

func (builder *builder) computeInputs(job config.Job, resourceOverrides config.Resources) ([]ProleBuilds.Input, error) {
	proleInputs := []ProleBuilds.Input{}
	for _, input := range job.Inputs {
		resource, found := resourceOverrides.Lookup(input.Resource)
		if !found {
			resource, found = builder.resources.Lookup(input.Resource)
			if !found {
				return nil, fmt.Errorf("unknown resource: %s", input.Resource)
			}

			if input.Passed != nil {
				outputs, err := builder.db.GetCommonOutputs(input.Passed, input.Resource)
				if err != nil {
					return nil, err
				}

				if len(outputs) == 0 {
					return nil, fmt.Errorf("unsatisfied input: %s; depends on %v\n", input.Resource, input.Passed)
				}

				resource.Source = outputs[len(outputs)-1]
			}
		}

		proleInput := ProleBuilds.Input{
			Type:   resource.Type,
			Source: ProleBuilds.Source(resource.Source),

			DestinationPath: resource.Name,
		}

		if filepath.HasPrefix(job.BuildConfigPath, resource.Name) {
			proleInput.ConfigPath = job.BuildConfigPath[len(resource.Name)+1:]
		}

		proleInputs = append(proleInputs, proleInput)
	}

	return proleInputs, nil
}
