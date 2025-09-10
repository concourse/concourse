package pipelineserver

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ListPipelineBuilds(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			err        error
			from       int
			to         int
			limit      int
			builds     []db.BuildForAPI
			pagination db.Pagination
		)

		logger := s.logger.Session("list-pipeline-builds")

		teamName := r.FormValue(":team_name")
		timestamps := r.FormValue(atc.PaginationQueryTimestamps)

		urlFrom := r.FormValue(atc.PaginationQueryFrom)
		urlTo := r.FormValue(atc.PaginationQueryTo)

		urlLimit := r.FormValue(atc.PaginationQueryLimit)

		limit, _ = strconv.Atoi(urlLimit)
		if limit == 0 {
			limit = atc.PaginationAPIDefaultLimit
		}

		page := db.Page{Limit: limit}
		if urlFrom != "" {
			from, _ = strconv.Atoi(urlFrom)
			page.From = db.NewIntPtr(from)
		}
		if urlTo != "" {
			to, _ = strconv.Atoi(urlTo)
			page.To = db.NewIntPtr(to)
		}

		if timestamps == "" {
			builds, pagination, err = pipeline.Builds(page)
			if err != nil {
				logger.Error("failed-to-get-pipeline-builds", err)
				w.WriteHeader(http.StatusNotFound)
				return
			}
		} else {
			page.UseDate = true
			builds, pagination, err = pipeline.BuildsWithTime(page)
			if err != nil {
				logger.Error("failed-to-get-pipeline-builds-in-range", err)
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}

		pipelineRef := atc.PipelineRef{
			Name:         pipeline.Name(),
			InstanceVars: pipeline.InstanceVars(),
		}
		if pagination.Older != nil {
			s.addNextLink(w, teamName, pipelineRef, *pagination.Older)
		}

		if pagination.Newer != nil {
			s.addPreviousLink(w, teamName, pipelineRef, *pagination.Newer)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		atc := make([]atc.Build, len(builds))
		for i := 0; i < len(builds); i++ {
			build := builds[i]
			atc[i] = present.Build(build, nil, nil)
		}

		err = sonic.ConfigDefault.NewEncoder(w).Encode(atc)
		if err != nil {
			logger.Error("failed-to-encode-builds", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}

func (s *Server) addNextLink(w http.ResponseWriter, teamName string, pipelineRef atc.PipelineRef, page db.Page) {
	if pipelineRef.InstanceVars != nil {
		w.Header().Add("Link", fmt.Sprintf(
			`<%s/api/v1/teams/%s/pipelines/%s/builds?%s=%d&%s=%d&%s>; rel="%s"`,
			s.externalURL,
			teamName,
			pipelineRef.Name,
			atc.PaginationQueryTo,
			*page.To,
			atc.PaginationQueryLimit,
			page.Limit,
			pipelineRef.QueryParams().Encode(),
			atc.LinkRelNext,
		))
	} else {
		w.Header().Add("Link", fmt.Sprintf(
			`<%s/api/v1/teams/%s/pipelines/%s/builds?%s=%d&%s=%d>; rel="%s"`,
			s.externalURL,
			teamName,
			pipelineRef.Name,
			atc.PaginationQueryTo,
			*page.To,
			atc.PaginationQueryLimit,
			page.Limit,
			atc.LinkRelNext,
		))
	}
}

func (s *Server) addPreviousLink(w http.ResponseWriter, teamName string, pipelineRef atc.PipelineRef, page db.Page) {
	if pipelineRef.InstanceVars != nil {
		w.Header().Add("Link", fmt.Sprintf(
			`<%s/api/v1/teams/%s/pipelines/%s/builds?%s=%d&%s=%d&%s>; rel="%s"`,
			s.externalURL,
			teamName,
			pipelineRef.Name,
			atc.PaginationQueryFrom,
			*page.From,
			atc.PaginationQueryLimit,
			page.Limit,
			pipelineRef.QueryParams().Encode(),
			atc.LinkRelPrevious,
		))
	} else {
		w.Header().Add("Link", fmt.Sprintf(
			`<%s/api/v1/teams/%s/pipelines/%s/builds?%s=%d&%s=%d>; rel="%s"`,
			s.externalURL,
			teamName,
			pipelineRef.Name,
			atc.PaginationQueryFrom,
			*page.From,
			atc.PaginationQueryLimit,
			page.Limit,
			atc.LinkRelPrevious,
		))
	}
}
