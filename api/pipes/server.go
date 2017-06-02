package pipes

import (
	"net/http"
	"sync"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/tedsuo/rata"
)

type Server struct {
	logger lager.Logger

	url         string
	externalURL string

	pipes  map[string]pipe
	pipesL *sync.RWMutex

	teamFactory db.TeamFactory
}

func NewServer(
	logger lager.Logger,
	url string,
	externalURL string,
	teamFactory db.TeamFactory,
) *Server {
	return &Server{
		logger: logger,

		url:         url,
		externalURL: externalURL,

		pipes:       make(map[string]pipe),
		pipesL:      new(sync.RWMutex),
		teamFactory: teamFactory,
	}
}

func (s *Server) forwardRequest(w http.ResponseWriter, r *http.Request, host string, route string, pipeID string) (*http.Response, error) {
	generator := rata.NewRequestGenerator(host, atc.Routes)

	req, err := generator.CreateRequest(
		route,
		rata.Params{"pipe_id": pipeID},
		r.Body,
	)

	if err != nil {
		return nil, err
	}

	req.Header = r.Header

	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}

	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return response, nil
}
