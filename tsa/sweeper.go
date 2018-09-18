package tsa

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"

	"net/http/httputil"

	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/tedsuo/rata"
)

const (
	SweepContainers = "sweep-containers"
	SweepVolumes    = "sweep-volumes"
)

type Sweeper struct {
	ATCEndpoint    *rata.RequestGenerator
	TokenGenerator TokenGenerator
}

func (l *Sweeper) Sweep(logger lager.Logger, worker atc.Worker, resourceAction string) ([]byte, error) {
	logger.Debug("start")
	defer logger.Debug("end")

	var (
		containerBytes []byte
		request        *http.Request
		err            error
	)
	switch resourceAction {
	case SweepContainers:
		request, err = l.ATCEndpoint.CreateRequest(atc.ListDestroyingContainers, nil, nil)
	case SweepVolumes:
		request, err = l.ATCEndpoint.CreateRequest(atc.ListDestroyingVolumes, nil, nil)
	default:
		return nil, errors.New(ResourceActionMissing)
	}

	if err != nil {
		logger.Error("failed-to-construct-request", err)
		return containerBytes, err
	}

	if worker.Name == "" {
		logger.Info("empty-worker-name-in-req")
		return containerBytes, fmt.Errorf("empty-worker-name")
	}

	request.URL.RawQuery = url.Values{
		"worker_name": []string{worker.Name},
	}.Encode()

	var jwtToken string
	jwtToken, err = l.TokenGenerator.GenerateSystemToken()

	if err != nil {
		logger.Error("failed-to-generate-token", err)
		return containerBytes, err
	}

	request.Header.Add("Authorization", "Bearer "+jwtToken)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		logger.Error(fmt.Sprintf("failed-to-%s", resourceAction), err)
		return containerBytes, err
	}

	logger.Debug("atc-response", lager.Data{"response-status": response.StatusCode})

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		logger.Error("bad-response", nil, lager.Data{
			"status-code": response.StatusCode,
		})

		b, _ := httputil.DumpResponse(response, true)
		return containerBytes, fmt.Errorf("bad-response (%d): %s", response.StatusCode, string(b))
	}

	containerBytes, err = ioutil.ReadAll(response.Body)
	if err != nil {
		logger.Error("failed-to-read-response-body", err)
		return containerBytes, fmt.Errorf("bad-repsonse-body (%d): %s", response.StatusCode, err.Error())
	}

	logger.Info(fmt.Sprintf("successfully-%s", resourceAction))
	return containerBytes, nil
}
