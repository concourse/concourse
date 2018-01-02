package legacyserver

import (
	"net/http"
	"net/url"

	"code.cloudfoundry.org/lager"
)

type Server struct {
	logger lager.Logger
}

func NewServer(
	logger lager.Logger,
) *Server {
	return &Server{
		logger: logger,
	}
}

func (s *Server) ListAuthMethods(w http.ResponseWriter, r *http.Request) {

	teamName := r.FormValue(":team_name")
	u := url.URL{
		Scheme:   r.URL.Scheme,
		Host:     r.URL.Host,
		Path:     "/auth/list_methods",
		RawQuery: "team_name=" + teamName,
	}

	http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
}

func (s *Server) GetAuthToken(w http.ResponseWriter, r *http.Request) {

	teamName := r.FormValue(":team_name")
	u := url.URL{
		Scheme:   r.URL.Scheme,
		Host:     r.URL.Host,
		Path:     "/auth/basic/token",
		RawQuery: "team_name=" + teamName,
	}

	http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
}

func (s *Server) GetUser(w http.ResponseWriter, r *http.Request) {

	u := url.URL{
		Scheme: r.URL.Scheme,
		Host:   r.URL.Host,
		Path:   "/auth/userinfo",
	}

	http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
}
