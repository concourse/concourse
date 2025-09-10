package versionserver

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"code.cloudfoundry.org/lager/v3"
	"github.com/bytedance/sonic"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ListResourceVersions(pipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("list-resource-versions")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			err   error
			from  int
			to    int
			limit int
		)

		err = r.ParseForm()
		if err != nil {
			logger.Error("failed-to-parse-request-form", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		fields := r.Form["filter"]
		versionFilter := make(atc.Version)
		for _, field := range fields {
			vs := strings.SplitN(field, ":", 2)
			if len(vs) == 2 {
				versionFilter[vs[0]] = vs[1]
			}
		}

		resourceName := r.FormValue(":resource_name")
		teamName := r.FormValue(":team_name")

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

		resource, found, err := pipeline.Resource(resourceName)
		if err != nil {
			logger.Error("failed-to-get-resource", err, lager.Data{"resource-name": resourceName})
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Info("resource-not-found", lager.Data{"resource-name": resourceName})
			w.WriteHeader(http.StatusNotFound)
			return
		}

		versions, pagination, found, err := resource.Versions(page, versionFilter)
		if err != nil {
			logger.Error("failed-to-get-resource-config-versions", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Info("resource-versions-not-found", lager.Data{"resource-name": resourceName})
			w.WriteHeader(http.StatusNotFound)
			return
		}

		pipelineRef := atc.PipelineRef{
			Name:         pipeline.Name(),
			InstanceVars: pipeline.InstanceVars(),
		}
		if pagination.Older != nil {
			s.addNextLink(w, teamName, pipelineRef, resourceName, *pagination.Older)
		}

		if pagination.Newer != nil {
			s.addPreviousLink(w, teamName, pipelineRef, resourceName, *pagination.Newer)
		}

		w.Header().Set("Content-Type", "application/json")

		w.WriteHeader(http.StatusOK)

		acc := accessor.GetAccessor(r)
		hideMetadata := !resource.Public() && !acc.IsAuthorized(teamName)

		versions = present.ResourceVersions(hideMetadata, versions)

		err = sonic.ConfigDefault.NewEncoder(w).Encode(versions)
		if err != nil {
			logger.Error("failed-to-encode-resource-versions", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}

func (s *Server) addNextLink(w http.ResponseWriter, teamName string, pipelineRef atc.PipelineRef, resourceName string, page db.Page) {
	if pipelineRef.InstanceVars != nil {
		w.Header().Add("Link", fmt.Sprintf(
			`<%s/api/v1/teams/%s/pipelines/%s/resources/%s/versions?%s=%d&%s=%d&%s>; rel="%s"`,
			s.externalURL,
			teamName,
			pipelineRef.Name,
			resourceName,
			atc.PaginationQueryTo,
			*page.To,
			atc.PaginationQueryLimit,
			page.Limit,
			pipelineRef.QueryParams().Encode(),
			atc.LinkRelNext,
		))
	} else {
		w.Header().Add("Link", fmt.Sprintf(
			`<%s/api/v1/teams/%s/pipelines/%s/resources/%s/versions?%s=%d&%s=%d>; rel="%s"`,
			s.externalURL,
			teamName,
			pipelineRef.Name,
			resourceName,
			atc.PaginationQueryTo,
			*page.To,
			atc.PaginationQueryLimit,
			page.Limit,
			atc.LinkRelNext,
		))
	}
}

func (s *Server) addPreviousLink(w http.ResponseWriter, teamName string, pipelineRef atc.PipelineRef, resourceName string, page db.Page) {
	if pipelineRef.InstanceVars != nil {
		w.Header().Add("Link", fmt.Sprintf(
			`<%s/api/v1/teams/%s/pipelines/%s/resources/%s/versions?%s=%d&%s=%d&%s>; rel="%s"`,
			s.externalURL,
			teamName,
			pipelineRef.Name,
			resourceName,
			atc.PaginationQueryFrom,
			*page.From,
			atc.PaginationQueryLimit,
			page.Limit,
			pipelineRef.QueryParams().Encode(),
			atc.LinkRelPrevious,
		))
	} else {
		w.Header().Add("Link", fmt.Sprintf(
			`<%s/api/v1/teams/%s/pipelines/%s/resources/%s/versions?%s=%d&%s=%d>; rel="%s"`,
			s.externalURL,
			teamName,
			pipelineRef.Name,
			resourceName,
			atc.PaginationQueryFrom,
			*page.From,
			atc.PaginationQueryLimit,
			page.Limit,
			atc.LinkRelPrevious,
		))
	}
}
