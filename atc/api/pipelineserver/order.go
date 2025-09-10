package pipelineserver

import (
	"errors"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager/v3"
	"github.com/bytedance/sonic"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) OrderPipelines(team db.Team) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("order-pipelines")

		var pipelinesNames []string
		if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&pipelinesNames); err != nil {
			logger.Error("invalid-json", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err := team.OrderPipelines(pipelinesNames)
		if err != nil {
			logger.Error("failed-to-order-pipelines", err, lager.Data{
				"pipeline_names": pipelinesNames,
			})
			var errNotFound db.ErrPipelineNotFound
			if errors.As(err, &errNotFound) {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintln(w, err.Error())
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
