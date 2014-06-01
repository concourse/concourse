package resources

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/tedsuo/router"
	ProleBuilds "github.com/winston-ci/prole/api/builds"
	"github.com/winston-ci/prole/routes"

	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
)

type ProleChecker struct {
	prole *router.RequestGenerator

	httpClient *http.Client
}

func NewProleChecker(prole *router.RequestGenerator) Checker {
	return &ProleChecker{
		prole: prole,

		httpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 5 * time.Minute,
			},
		},
	}
}

func (checker *ProleChecker) CheckResource(resource config.Resource, from builds.Version) []builds.Version {
	req := new(bytes.Buffer)

	buildInput := ProleBuilds.Input{
		Type:    resource.Type,
		Source:  ProleBuilds.Source(resource.Source),
		Version: ProleBuilds.Version(from),
	}

	err := json.NewEncoder(req).Encode(buildInput)
	if err != nil {
		log.Println("encoding input failed:", err)
		return nil
	}

	check, err := checker.prole.RequestForHandler(
		routes.CheckInput,
		nil,
		req,
	)
	if err != nil {
		log.Println("constructing check request failed:", err)
		return nil
	}

	check.Header.Set("Content-Type", "application/json")

	resp, err := checker.httpClient.Do(check)
	if err != nil {
		log.Println("prole request failed:", err)
		return nil
	}

	defer resp.Body.Close()

	var newVersions []builds.Version
	err = json.NewDecoder(resp.Body).Decode(&newVersions)
	if err != nil {
		log.Println("invalid check response:", err)
		return nil
	}

	return newVersions
}
