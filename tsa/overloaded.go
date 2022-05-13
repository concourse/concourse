package tsa

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/tedsuo/rata"
)

type OverloadedStatus struct {
	ATCEndpoint *rata.RequestGenerator
	HTTPClient  *http.Client
	Overloaded  bool
}

func (o *OverloadedStatus) SetOverload(ctx context.Context, worker atc.Worker) error {
	logger := lagerctx.FromContext(ctx)

	logger.Info("start")
	defer logger.Info("end")

	request, err := o.ATCEndpoint.CreateRequest(atc.WorkerOverloaded, rata.Params{
		"worker_name": worker.Name,
	},
		strings.NewReader("true"))
	if err != nil {
		logger.Error("failed-to-construct-request", err)
		return err
	}

	response, err := o.HTTPClient.Do(request)
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
