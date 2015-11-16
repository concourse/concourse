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
			builds  []db.Build
			err     error
			until   int
			since   int
			limit   int
			hasMore bool
		)

		jobName := r.FormValue(":job_name")

		urlUntil := r.FormValue("until")
		if urlUntil != "" {
			until, err = strconv.Atoi(urlUntil)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		urlSince := r.FormValue("since")
		if urlSince != "" {
			since, err = strconv.Atoi(urlSince)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		urlLimit := r.FormValue("limit")
		if urlLimit != "" {
			limit, err = strconv.Atoi(urlLimit)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			limit = 100
		}

		if until == 0 && since == 0 {
			builds, err = pipelineDB.GetAllJobBuilds(jobName)
		} else if until != 0 {
			builds, hasMore, err = pipelineDB.GetJobBuildsCursor(jobName, until, false, limit)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			if len(builds) > 0 && hasMore {
				s.addNextLink(w, builds[len(builds)-1].ID-1, pipelineDB.GetPipelineName(), jobName)
			}

			maxID, err := pipelineDB.GetJobBuildsMaxID(jobName)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if maxID > until {
				s.addPreviousLink(w, until+1, pipelineDB.GetPipelineName(), jobName)
			}
		} else {
			builds, hasMore, err = pipelineDB.GetJobBuildsCursor(jobName, since, true, limit)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			if len(builds) > 0 && hasMore {
				s.addPreviousLink(w, builds[0].ID+1, pipelineDB.GetPipelineName(), jobName)
			}

			minID, err := pipelineDB.GetJobBuildsMinID(jobName)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if minID < since {
				s.addNextLink(w, since-1, pipelineDB.GetPipelineName(), jobName)
			}
		}

		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusOK)

		resources := make([]atc.Build, len(builds))
		for i := 0; i < len(builds); i++ {
			resources[i] = present.Build(builds[i])
		}
		json.NewEncoder(w).Encode(resources)
	})
}

func (s *Server) addNextLink(w http.ResponseWriter, targetID int, pipelineName string, jobName string) {
	w.Header().Add("LINK", fmt.Sprintf(
		`<%s/api/v1/pipelines/%s/jobs/%s/builds?until=%d>; rel="next"`,
		s.externalURL,
		pipelineName,
		jobName,
		targetID,
	))
}

func (s *Server) addPreviousLink(w http.ResponseWriter, targetID int, pipelineName string, jobName string) {
	w.Header().Add("LINK", fmt.Sprintf(
		`<%s/api/v1/pipelines/%s/jobs/%s/builds?since=%d>; rel="previous"`,
		s.externalURL,
		pipelineName,
		jobName,
		targetID,
	))
}
