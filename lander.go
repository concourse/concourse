package tsa

import (
	"net/http"

	"net/http/httputil"

	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/tedsuo/rata"
)

type Lander struct {
	ATCEndpoint    *rata.RequestGenerator
	TokenGenerator TokenGenerator
}

func (l *Lander) Land(logger lager.Logger, worker atc.Worker) error {
	logger.Info("start")
	defer logger.Info("end")

	request, err := l.ATCEndpoint.CreateRequest(atc.LandWorker, rata.Params{
		"worker_name": worker.Name,
	}, nil)
	if err != nil {
		logger.Error("failed-to-construct-request", err)
		return err
	}

	jwtToken, err := l.TokenGenerator.GenerateToken()
	if err != nil {
		logger.Error("failed-to-construct-request", err)
		return err
	}

	request.Header.Add("Authorization", "Bearer "+jwtToken)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		logger.Error("failed-to-land", err)
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
