package tsa

import (
	"net/http"

	"net/http/httputil"

	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/tedsuo/rata"
)

type Retirer struct {
	ATCEndpoint    *rata.RequestGenerator
	TokenGenerator TokenGenerator
}

func (l *Retirer) Retire(logger lager.Logger, worker atc.Worker) error {
	logger.Info("start")
	defer logger.Info("end")

	request, err := l.ATCEndpoint.CreateRequest(atc.RetireWorker, rata.Params{
		"worker_name": worker.Name,
	}, nil)
	if err != nil {
		logger.Error("failed-to-construct-request", err)
		return err
	}

	var jwtToken string
	if worker.Team != "" {
		jwtToken, err = l.TokenGenerator.GenerateTeamToken(worker.Team)
	} else {
		jwtToken, err = l.TokenGenerator.GenerateSystemToken()
	}
	if err != nil {
		logger.Error("failed-to-generate-token", err)
		return err
	}

	request.Header.Add("Authorization", "Bearer "+jwtToken)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		logger.Error("failed-to-retire", err)
		return err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		logger.Info("worker-not-found")
		return nil
	}

	if response.StatusCode != http.StatusOK {
		logger.Error("bad-response", nil, lager.Data{
			"status-code": response.StatusCode,
		})

		b, _ := httputil.DumpResponse(response, true)
		return fmt.Errorf("bad-response (%d): %s", response.StatusCode, string(b))
	}

	return nil
}
