package jobserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
)

func (s *Server) ListJobBuilds(pipelineDB db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			builds []db.Build
			err    error
			until  int
			since  int
			limit  int
		)

		jobName := r.FormValue(":job_name")

		urlUntil := r.FormValue(atc.PaginationQueryUntil)
		if urlUntil != "" {
			until, err = strconv.Atoi(urlUntil)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		urlSince := r.FormValue(atc.PaginationQuerySince)
		if urlSince != "" {
			since, err = strconv.Atoi(urlSince)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		urlLimit := r.FormValue(atc.PaginationQueryLimit)
		if urlLimit != "" {
			limit, err = strconv.Atoi(urlLimit)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			limit = atc.PaginationAPIDefaultLimit
		}

		page := db.Page{
			Since: since,
			Until: until,
			Limit: limit,
		}

		builds, pagination, err := pipelineDB.GetJobBuilds(jobName, page)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if pagination.Next != nil {
			s.addNextLink(w, pipelineDB.GetPipelineName(), jobName, *pagination.Next)
		}

		if pagination.Previous != nil {
			s.addPreviousLink(w, pipelineDB.GetPipelineName(), jobName, *pagination.Previous)
		}

		w.WriteHeader(http.StatusOK)

		resources := make([]atc.Build, len(builds))
		for i := 0; i < len(builds); i++ {
			resources[i] = present.Build(builds[i])
		}
		json.NewEncoder(w).Encode(resources)
	})
}

func (s *Server) addNextLink(w http.ResponseWriter, pipelineName string, jobName string, page db.Page) {
	w.Header().Add("Link", fmt.Sprintf(
		`<%s/api/v1/pipelines/%s/jobs/%s/builds?%s=%d&%s=%d>; rel="%s"`,
		s.externalURL,
		pipelineName,
		jobName,
		atc.PaginationQuerySince,
		page.Since,
		atc.PaginationQueryLimit,
		page.Limit,
		atc.LinkRelNext,
	))
}

func (s *Server) addPreviousLink(w http.ResponseWriter, pipelineName string, jobName string, page db.Page) {
	w.Header().Add("Link", fmt.Sprintf(
		`<%s/api/v1/pipelines/%s/jobs/%s/builds?%s=%d&%s=%d>; rel="%s"`,
		s.externalURL,
		pipelineName,
		jobName,
		atc.PaginationQueryUntil,
		page.Until,
		atc.PaginationQueryLimit,
		page.Limit,
		atc.LinkRelPrevious,
	))
}
