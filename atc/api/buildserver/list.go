package buildserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ListBuilds(w http.ResponseWriter, r *http.Request) {

	logger := s.logger.Session("list-builds")

	var (
		err     error
		from    int
		to      int
		limit   int
		useDate bool
	)

	timestamps := r.FormValue(atc.PaginationQueryTimestamps)
	if timestamps != "" {
		useDate = true
	}

	urlFrom := r.FormValue(atc.PaginationQueryFrom)
	from, _ = strconv.Atoi(urlFrom)

	urlTo := r.FormValue(atc.PaginationQueryTo)
	to, _ = strconv.Atoi(urlTo)

	urlLimit := r.FormValue(atc.PaginationQueryLimit)
	limit, _ = strconv.Atoi(urlLimit)
	if limit == 0 {
		limit = atc.PaginationAPIDefaultLimit
	}

	page := db.Page{From: from, To: to, Limit: limit, UseDate: useDate}

	var builds []db.Build
	var pagination db.Pagination

	acc := accessor.GetAccessor(r)
	if acc.IsAdmin() {
		builds, pagination, err = s.buildFactory.AllBuilds(page)
	} else {
		builds, pagination, err = s.buildFactory.VisibleBuilds(acc.TeamNames(), page)
	}

	if err != nil {
		logger.Error("failed-to-get-all-builds", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if pagination.Older != nil {
		s.addNextLink(w, *pagination.Older)
	}

	if pagination.Newer != nil {
		s.addPreviousLink(w, *pagination.Newer)
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

func (s *Server) addNextLink(w http.ResponseWriter, page db.Page) {
	w.Header().Add("Link", fmt.Sprintf(
		`<%s/api/v1/builds?%s=%d&%s=%d>; rel="%s"`,
		s.externalURL,
		atc.PaginationQueryTo,
		page.To,
		atc.PaginationQueryLimit,
		page.Limit,
		atc.LinkRelNext,
	))
}

func (s *Server) addPreviousLink(w http.ResponseWriter, page db.Page) {
	w.Header().Add("Link", fmt.Sprintf(
		`<%s/api/v1/builds?%s=%d&%s=%d>; rel="%s"`,
		s.externalURL,
		atc.PaginationQueryFrom,
		page.From,
		atc.PaginationQueryLimit,
		page.Limit,
		atc.LinkRelPrevious,
	))
}
