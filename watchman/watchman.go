package watchman

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/tedsuo/router"
	"github.com/winston-ci/prole/api/builds"
	"github.com/winston-ci/prole/routes"

	"github.com/winston-ci/winston/builder"
	"github.com/winston-ci/winston/config"
)

type Watchman interface {
	Watch(job config.Job, input config.Input, interval time.Duration) (stop chan<- struct{})
}

type watchman struct {
	builder builder.Builder
	prole   *router.RequestGenerator
}

func NewWatchman(builder builder.Builder, prole *router.RequestGenerator) Watchman {
	return &watchman{
		builder: builder,
		prole:   prole,
	}
}

func (watchman *watchman) Watch(
	job config.Job,
	inputConfig config.Input,
	interval time.Duration,
) chan<- struct{} {
	stop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				for _, source := range watchman.check(inputConfig) {
					inputConfig.Source = config.Source(source)

					watchman.builder.Build(job.UpdateInput(inputConfig))
				}
			}
		}
	}()

	return stop
}

func (watchman *watchman) check(inputConfig config.Input) []builds.Source {
	req := new(bytes.Buffer)

	input := builds.Input{
		Type:   inputConfig.Type,
		Source: builds.Source(inputConfig.Source),
	}

	err := json.NewEncoder(req).Encode(input)
	if err != nil {
		log.Println("encoding input failed:", err)
		return nil
	}

	check, err := watchman.prole.RequestForHandler(
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

	var newSources []builds.Source
	err = json.NewDecoder(resp.Body).Decode(&newSources)
	if err != nil {
		log.Println("invalid check response:", err)
		return nil
	}

	resp.Body.Close()

	return newSources
}
