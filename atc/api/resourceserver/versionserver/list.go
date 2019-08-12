package versionserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"code.cloudfoundry.org/lager"
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
			until int
			since int
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

		urlUntil := r.FormValue(atc.PaginationQueryUntil)
		until, _ = strconv.Atoi(urlUntil)

		urlSince := r.FormValue(atc.PaginationQuerySince)
		since, _ = strconv.Atoi(urlSince)

		urlFrom := r.FormValue(atc.PaginationQueryFrom)
		from, _ = strconv.Atoi(urlFrom)

		urlTo := r.FormValue(atc.PaginationQueryTo)
		to, _ = strconv.Atoi(urlTo)

		urlLimit := r.FormValue(atc.PaginationQueryLimit)

		limit, _ = strconv.Atoi(urlLimit)
		if limit == 0 {
			limit = atc.PaginationAPIDefaultLimit
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

		versions, pagination, found, err := resource.Versions(db.Page{
			Until: until,
			Since: since,
			From:  from,
			To:    to,
			Limit: limit,
		}, versionFilter)
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

		if pagination.Next != nil {
			s.addNextLink(w, teamName, pipeline.Name(), resourceName, *pagination.Next)
		}

		if pagination.Previous != nil {
			s.addPreviousLink(w, teamName, pipeline.Name(), resourceName, *pagination.Previous)
		}

		w.Header().Set("Content-Type", "application/json")

		w.WriteHeader(http.StatusOK)

		acc := accessor.GetAccessor(r)
		hideMetadata := !resource.Public() && !acc.IsAuthorized(teamName)

		versions = present.ResourceVersions(hideMetadata, versions)

		err = json.NewEncoder(w).Encode(versions)
		if err != nil {
			logger.Error("failed-to-encode-resource-versions", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}

func (s *Server) addNextLink(w http.ResponseWriter, teamName, pipelineName, resourceName string, page db.Page) {
	w.Header().Add("Link", fmt.Sprintf(
		`<%s/api/v1/teams/%s/pipelines/%s/resources/%s/versions?%s=%d&%s=%d>; rel="%s"`,
		s.externalURL,
		teamName,
		pipelineName,
		resourceName,
		atc.PaginationQuerySince,
		page.Since,
		atc.PaginationQueryLimit,
		page.Limit,
		atc.LinkRelNext,
	))
}

func (s *Server) addPreviousLink(w http.ResponseWriter, teamName, pipelineName, resourceName string, page db.Page) {
	w.Header().Add("Link", fmt.Sprintf(
		`<%s/api/v1/teams/%s/pipelines/%s/resources/%s/versions?%s=%d&%s=%d>; rel="%s"`,
		s.externalURL,
		teamName,
		pipelineName,
		resourceName,
		atc.PaginationQueryUntil,
		page.Until,
		atc.PaginationQueryLimit,
		page.Limit,
		atc.LinkRelPrevious,
	))
}
