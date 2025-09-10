package teamserver

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ListTeamBuilds(team db.Team) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			from       int
			to         int
			limit      int
			builds     []db.BuildForAPI
			pagination db.Pagination
			err        error
		)

		logger := s.logger.Session("list-team-builds")

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
			builds, pagination, err = team.Builds(page)
		} else {
			builds, pagination, err = team.BuildsWithTime(page)
		}
		if err != nil {
			logger.Error("failed-to-get-team-builds", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if pagination.Older != nil {
			s.addNextLink(w, teamName, *pagination.Older)
		}

		if pagination.Newer != nil {
			s.addPreviousLink(w, teamName, *pagination.Newer)
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

func (s *Server) addNextLink(w http.ResponseWriter, teamName string, page db.Page) {
	w.Header().Add("Link", fmt.Sprintf(
		`<%s/api/v1/teams/%s/builds?%s=%d&%s=%d>; rel="%s"`,
		s.externalURL,
		teamName,
		atc.PaginationQueryTo,
		*page.To,
		atc.PaginationQueryLimit,
		page.Limit,
		atc.LinkRelNext,
	))
}

func (s *Server) addPreviousLink(w http.ResponseWriter, teamName string, page db.Page) {
	w.Header().Add("Link", fmt.Sprintf(
		`<%s/api/v1/teams/%s/builds?%s=%d&%s=%d>; rel="%s"`,
		s.externalURL,
		teamName,
		atc.PaginationQueryFrom,
		*page.From,
		atc.PaginationQueryLimit,
		page.Limit,
		atc.LinkRelPrevious,
	))
}
