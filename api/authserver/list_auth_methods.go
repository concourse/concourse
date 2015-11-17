package authserver

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/web"
	"github.com/tedsuo/rata"
)

const BasicAuthDisplayName = "Basic Auth"

func (s *Server) ListAuthMethods(w http.ResponseWriter, r *http.Request) {
	methods := []atc.AuthMethod{}

	for name, provider := range s.providers {
		path, err := auth.OAuthRoutes.CreatePathForRoute(
			auth.OAuthBegin,
			rata.Params{"provider": name},
		)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		methods = append(methods, atc.AuthMethod{
			Type:        atc.AuthTypeOAuth,
			DisplayName: provider.DisplayName(),
			AuthURL:     s.externalURL + path,
		})
	}

	if s.basicAuthEnabled {
		path, err := web.Routes.CreatePathForRoute(
			web.BasicAuth,
			rata.Params{},
		)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		methods = append(methods, atc.AuthMethod{
			Type:        atc.AuthTypeBasic,
			DisplayName: BasicAuthDisplayName,
			AuthURL:     s.externalURL + path,
		})
	}

	sort.Sort(byTypeAndName(methods))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(methods)
}

type byTypeAndName []atc.AuthMethod

func (ms byTypeAndName) Len() int          { return len(ms) }
func (ms byTypeAndName) Swap(i int, j int) { ms[i], ms[j] = ms[j], ms[i] }
func (ms byTypeAndName) Less(i int, j int) bool {
	if ms[i].Type == atc.AuthTypeBasic && ms[j].Type == atc.AuthTypeOAuth {
		return false
	}

	if ms[i].Type == atc.AuthTypeOAuth && ms[j].Type == atc.AuthTypeBasic {
		return true
	}

	return ms[i].DisplayName < ms[j].DisplayName
}
