package builder

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/tedsuo/router"
	ProleBuilds "github.com/winston-ci/prole/api/builds"
	ProleRoutes "github.com/winston-ci/prole/routes"

	WinstonRoutes "github.com/winston-ci/winston/api/routes"
	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/jobs"
)

var ErrBadResponse = errors.New("bad response from prole")

type Builder interface {
	Build(jobs.Job) (builds.Build, error)
}

type builder struct {
	db db.DB

	prole   *router.RequestGenerator
	winston *router.RequestGenerator
}

func NewBuilder(
	db db.DB,
	prole *router.RequestGenerator,
	winston *router.RequestGenerator,
) Builder {
	return &builder{
		db: db,

		prole:   prole,
		winston: winston,
	}
}

func (builder *builder) Build(job jobs.Job) (builds.Build, error) {
	log.Println("creating build")

	build, err := builder.db.CreateBuild(job.Name)
	if err != nil {
		return builds.Build{}, err
	}

	var source ProleBuilds.BuildSource
	for _, resource := range job.Inputs {
		source = resource.BuildSource()
	}

	complete, err := builder.winston.RequestForHandler(
		WinstonRoutes.SetResult,
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
		ConfigPath: job.BuildConfigPath,

		Source: source,

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

	log.Println("build running")

	return builder.db.SaveBuildState(job.Name, build.ID, builds.BuildStateRunning)
}
