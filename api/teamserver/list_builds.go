package teamserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
)

func (s *Server) ListTeamBuilds(w http.ResponseWriter, r *http.Request) {
	var (
		until      int
		since      int
		limit      int
		builds     []db.Build
		pagination db.Pagination
	)

	logger := s.logger.Session("list-team-builds")

	teamName := r.FormValue(":team_name")

	urlUntil := r.FormValue(atc.PaginationQueryUntil)
	until, _ = strconv.Atoi(urlUntil)

	urlSince := r.FormValue(atc.PaginationQuerySince)
	since, _ = strconv.Atoi(urlSince)

	urlLimit := r.FormValue(atc.PaginationQueryLimit)

	limit, _ = strconv.Atoi(urlLimit)
	if limit == 0 {
		limit = atc.PaginationAPIDefaultLimit
	}

	page := db.Page{Until: until, Since: since, Limit: limit}

	team, found, err := s.teamFactory.FindTeam(teamName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	builds, pagination, err = team.Builds(page)
	if err != nil {
		logger.Error("failed-to-get-team-builds", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if pagination.Next != nil {
		s.addNextLink(w, teamName, *pagination.Next)
	}

	if pagination.Previous != nil {
		s.addPreviousLink(w, teamName, *pagination.Previous)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	atc := make([]atc.Build, len(builds))
	for i := 0; i < len(builds); i++ {
		build := builds[i]
		atc[i] = present.Build(build)
	}

	err = json.NewEncoder(w).Encode(atc)
	if err != nil {
		logger.Error("failed-to-encode-builds", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Server) addNextLink(w http.ResponseWriter, teamName string, page db.Page) {
	w.Header().Add("Link", fmt.Sprintf(
		`<%s/api/v1/teams/%s/builds?%s=%d&%s=%d>; rel="%s"`,
		s.externalURL,
		teamName,
		atc.PaginationQuerySince,
		page.Since,
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
		atc.PaginationQueryUntil,
		page.Until,
		atc.PaginationQueryLimit,
		page.Limit,
		atc.LinkRelPrevious,
	))
}
