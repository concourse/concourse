package resourceserver

import (
	"context"
	"net/http"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerctx"
	"github.com/bytedance/sonic"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
	"github.com/tedsuo/rata"
)

func (s *Server) CheckResource(dbPipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceName := rata.Param(r, "resource_name")

		logger := s.logger.Session("check-resource", lager.Data{
			"resource": resourceName,
		})

		var reqBody atc.CheckRequestBody
		err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&reqBody)
		if err != nil {
			logger.Info("malformed-request", lager.Data{"error": err.Error()})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		dbResource, found, err := dbPipeline.Resource(resourceName)
		if err != nil {
			logger.Error("failed-to-get-resource", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Info("resource-not-found")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		dbResourceTypes, err := dbPipeline.ResourceTypes()
		if err != nil {
			logger.Error("failed-to-get-resource-types", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		build, created, err := s.checkFactory.TryCreateCheck(
			lagerctx.NewContext(context.Background(), logger),
			dbResource,
			dbResourceTypes,
			reqBody.From,
			true,
			!reqBody.Shallow,
			true,
		)
		if err != nil {
			logger.Error("failed-to-create-check", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		if !created {
			logger.Info("check-not-created")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)

		err = sonic.ConfigDefault.NewEncoder(w).Encode(present.Build(build, nil, nil))
		if err != nil {
			logger.Error("failed-to-encode-check", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
