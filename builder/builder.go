package builder

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/tedsuo/rata"

	"github.com/concourse/atc/db"
	"github.com/concourse/turbine"
)

var ErrBadResponse = errors.New("bad response from turbine")

type BuilderDB interface {
	StartBuild(buildID int, abortURL, hijackURL string) (bool, error)
}

type Builder interface {
	Build(db.Build, turbine.Build) error
}

type builder struct {
	db BuilderDB

	turbine *rata.RequestGenerator

	httpClient *http.Client
}

func NewBuilder(db BuilderDB, turbine *rata.RequestGenerator) Builder {
	return &builder{
		db: db,

		turbine: turbine,

		httpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 5 * time.Minute,

				// allow DNS to resolve differently every time
				DisableKeepAlives: true,
			},
		},
	}
}

func (builder *builder) Build(build db.Build, turbineBuild turbine.Build) error {
	req := new(bytes.Buffer)

	err := json.NewEncoder(req).Encode(turbineBuild)
	if err != nil {
		return err
	}

	execute, err := builder.turbine.CreateRequest(
		turbine.ExecuteBuild,
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

	if resp.StatusCode != http.StatusCreated {
		return ErrBadResponse
	}

	var startedBuild turbine.Build
	err = json.NewDecoder(resp.Body).Decode(&startedBuild)
	if err != nil {
		return err
	}

	resp.Body.Close()

	started, err := builder.db.StartBuild(build.ID, startedBuild.Guid, resp.Header.Get("X-Turbine-Endpoint"))
	if err != nil {
		return err
	}

	if !started {
		builder.abort(startedBuild.Guid)
	}

	return nil
}

func (builder *builder) abort(guid string) error {
	abort, err := builder.turbine.CreateRequest(
		turbine.AbortBuild,
		rata.Params{"guid": guid},
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := builder.httpClient.Do(abort)
	if err == nil {
		resp.Body.Close()
	}

	return err
}
