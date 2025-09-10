package resourceserver

import (
	"net/http"

	"code.cloudfoundry.org/lager/v3"
	"github.com/bytedance/sonic"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) SetPinCommentOnResource(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceName := r.FormValue(":resource_name")

		logger := s.logger.Session("set-pin-comment-on-resource", lager.Data{
			"resource": resourceName,
		})

		resource, found, err := pipeline.Resource(resourceName)
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

		var reqBody atc.SetPinCommentRequestBody
		err = sonic.ConfigDefault.NewDecoder(r.Body).Decode(&reqBody)
		if err != nil {
			logger.Info("malformed-request", lager.Data{"error": err.Error()})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = resource.SetPinComment(reqBody.PinComment)
		if err != nil {
			logger.Error("failed-to-set-pin-comment-on-resource", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
