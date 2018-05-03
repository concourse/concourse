package tsa

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"

	"net/http/httputil"

	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/tedsuo/rata"
)

type WorkerStatus struct {
	ATCEndpoint      *rata.RequestGenerator
	TokenGenerator   TokenGenerator
	ContainerHandles []string
}

func (l *WorkerStatus) WorkerStatus(logger lager.Logger, worker atc.Worker) error {
	logger.Debug("start")
	defer logger.Debug("end")

	handlesBytes, err := json.Marshal(l.ContainerHandles)
	if err != nil {
		logger.Error("failed-to-encode-request-body", err)
		return err
	}

	request, err := l.ATCEndpoint.CreateRequest(atc.ReportWorkerContainers, nil, bytes.NewBuffer(handlesBytes))

	if err != nil {
		logger.Error("failed-to-construct-request", err)
		return err
	}

	if worker.Name == "" {
		logger.Info("empty-worker-name-in-req")
		return fmt.Errorf("empty-worker-name")
	}
	request.Header.Add("Content-Type", "application/json")

	request.URL.RawQuery = url.Values{
		"worker_name": []string{worker.Name},
	}.Encode()

	var jwtToken string
	jwtToken, err = l.TokenGenerator.GenerateSystemToken()

	if err != nil {
		logger.Error("failed-to-generate-token", err)
		return err
	}

	request.Header.Add("Authorization", "Bearer "+jwtToken)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		logger.Error("failed-to-collect-containers", err)
		return err
	}

	logger.Debug("atc-response", lager.Data{"response-status": response.StatusCode})

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		logger.Error("bad-response", nil, lager.Data{
			"status-code": response.StatusCode,
		})

		b, _ := httputil.DumpResponse(response, true)
		return fmt.Errorf("bad-response (%d): %s", response.StatusCode, string(b))
	}

	logger.Info("successfully-sweeped-containers")
	return nil
}
