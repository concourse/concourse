package resourceserver

import (
	"context"
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
	"github.com/tedsuo/rata"
)

func (s *Server) CheckPrototype(dbPipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prototypeName := rata.Param(r, "prototype_name")

		logger := s.logger.Session("check-prototype", lager.Data{
			"prototype": prototypeName,
		})

		var reqBody atc.CheckRequestBody
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		if err != nil {
			logger.Info("malformed-request", lager.Data{"error": err.Error()})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		dbPrototype, found, err := dbPipeline.Prototype(prototypeName)
		if err != nil {
			logger.Error("failed-to-get-prototype", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Info("prototype-not-found")
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
			dbPrototype,
			dbResourceTypes,
			reqBody.From,
			true,
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

		err = json.NewEncoder(w).Encode(present.Build(build, nil, nil))
		if err != nil {
			logger.Error("failed-to-encode-check", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
