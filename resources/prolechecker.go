package resources

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"

	"github.com/tedsuo/router"
	"github.com/winston-ci/prole/api/builds"
	"github.com/winston-ci/prole/routes"
	"github.com/winston-ci/winston/config"
)

type ProleChecker struct {
	prole *router.RequestGenerator
}

func NewProleChecker(prole *router.RequestGenerator) Checker {
	return &ProleChecker{prole}
}

func (checker *ProleChecker) CheckResource(resource config.Resource) []config.Resource {
	req := new(bytes.Buffer)

	buildInput := builds.Input{
		Type:   resource.Type,
		Source: builds.Source(resource.Source),
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

	resp, err := http.DefaultClient.Do(check)
	if err != nil {
		log.Println("prole request failed:", err)
		return nil
	}

	defer resp.Body.Close()

	var newSources []builds.Source
	err = json.NewDecoder(resp.Body).Decode(&newSources)
	if err != nil {
		log.Println("invalid check response:", err)
		return nil
	}

	newResources := make([]config.Resource, len(newSources))
	for i, source := range newSources {
		newResource := resource
		newResource.Source = config.Source(source)
		newResources[i] = newResource
	}

	return newResources
}
