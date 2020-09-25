package tsa

import (
	"context"
	"net/http"

	"net/http/httputil"

	"fmt"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
)

type Retirer struct {
	ATCEndpoint atc.Endpoint
	HTTPClient  *http.Client
}

func (l *Retirer) Retire(ctx context.Context, worker atc.Worker) error {
	logger := lagerctx.FromContext(ctx)

	logger.Info("start")
	defer logger.Info("end")

	request, err := l.ATCEndpoint.CreateRequest(atc.RetireWorker, map[string]string{
		"worker_name": worker.Name,
	}, nil)
	if err != nil {
		logger.Error("failed-to-construct-request", err)
		return err
	}

	response, err := l.HTTPClient.Do(request)
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
