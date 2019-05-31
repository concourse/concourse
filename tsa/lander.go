package tsa

import (
	"context"
	"net/http"

	"net/http/httputil"

	"fmt"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/v5/atc"
	"github.com/tedsuo/rata"
)

type Lander struct {
	ATCEndpoint    *rata.RequestGenerator
	TokenGenerator TokenGenerator
}

func (l *Lander) Land(ctx context.Context, worker atc.Worker) error {
	logger := lagerctx.FromContext(ctx)

	logger.Info("start")
	defer logger.Info("end")

	request, err := l.ATCEndpoint.CreateRequest(atc.LandWorker, rata.Params{
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
		logger.Error("failed-to-land", err)
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		logger.Error("bad-response", nil, lager.Data{
			"status-code": response.StatusCode,
		})

		b, _ := httputil.DumpResponse(response, true)
		return fmt.Errorf("bad-response (%d): %s", response.StatusCode, string(b))
	}

	return nil
}
