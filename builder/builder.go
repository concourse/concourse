package builder

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	tbuilds "github.com/concourse/turbine/api/builds"
	troutes "github.com/concourse/turbine/routes"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/builds"
	croutes "github.com/concourse/atc/callbacks/routes"
)

var ErrBadResponse = errors.New("bad response from turbine")

type BuilderDB interface {
	StartBuild(buildID int, abortURL string) (bool, error)
}

type Builder interface {
	Build(builds.Build, tbuilds.Build) error
}

type builder struct {
	db BuilderDB

	turbine *rata.RequestGenerator
	atc     *rata.RequestGenerator

	httpClient *http.Client
}

func NewBuilder(db BuilderDB, turbine *rata.RequestGenerator, atc *rata.RequestGenerator) Builder {
	return &builder{
		db: db,

		turbine: turbine,
		atc:     atc,

		httpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 5 * time.Minute,
			},
		},
	}
}

func (builder *builder) Build(build builds.Build, turbineBuild tbuilds.Build) error {
	updateBuild, err := builder.atc.CreateRequest(
		croutes.UpdateBuild,
		rata.Params{
			"build": fmt.Sprintf("%d", build.ID),
		},
		nil,
	)
	if err != nil {
		panic(err)
	}

	turbineBuild.StatusCallback = updateBuild.URL.String()

	recordEvents, err := builder.atc.CreateRequest(
		croutes.RecordEvents,
		rata.Params{
			"build": fmt.Sprintf("%d", build.ID),
		},
		nil,
	)
	if err != nil {
		panic(err)
	}

	recordEvents.URL.Scheme = "ws"

	turbineBuild.EventsCallback = recordEvents.URL.String()

	req := new(bytes.Buffer)

	err = json.NewEncoder(req).Encode(turbineBuild)
	if err != nil {
		return err
	}

	execute, err := builder.turbine.CreateRequest(
		troutes.ExecuteBuild,
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

	var startedBuild tbuilds.Build
	err = json.NewDecoder(resp.Body).Decode(&startedBuild)
	if err != nil {
		return err
	}

	resp.Body.Close()

	started, err := builder.db.StartBuild(build.ID, startedBuild.AbortURL)
	if err != nil {
		return err
	}

	if !started {
		builder.httpClient.Post(startedBuild.AbortURL, "", nil)
	}

	return nil
}
