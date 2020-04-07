package legacyserver

import (
	"net/http"
	"net/url"

	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/rata"
)

type LegacyConfig struct {
	Logger lager.Logger
}

const (
	LoginRoute    = "LoginRoute"
	LogoutRoute   = "LogoutRoute"
	CallbackRoute = "CallbackRoute"
)

func NewLegacyServer(config *LegacyConfig) (http.Handler, error) {

	routes := rata.Routes([]rata.Route{
		{Path: "/login", Method: "GET", Name: LoginRoute},
		{Path: "/logout", Method: "GET", Name: LogoutRoute},
		{Path: "/auth/:provider/callback", Method: "GET", Name: CallbackRoute},
	})

	handlers := map[string]http.Handler{

		LoginRoute: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := url.URL{
				Scheme:   r.URL.Scheme,
				Host:     r.URL.Host,
				Path:     "/sky/login",
				RawQuery: r.URL.RawQuery,
			}

			flyPort := r.FormValue("fly_port")
			if flyPort != "" {
				u.RawQuery = "redirect_uri=/fly_success%3Ffly_port=" + flyPort
			}

			http.Redirect(w, r, u.String(), http.StatusFound)
		}),

		LogoutRoute: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := url.URL{
				Scheme: r.URL.Scheme,
				Host:   r.URL.Host,
				Path:   "/sky/logout",
			}

			http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
		}),

		CallbackRoute: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			q, _ := url.ParseQuery(r.URL.RawQuery)
			q.Del(":provider")

			u := url.URL{
				Scheme:   r.URL.Scheme,
				Host:     r.URL.Host,
				Path:     "/sky/issuer/callback",
				RawQuery: q.Encode(),
			}

			http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
		}),
	}

	return rata.NewRouter(routes, handlers)
}
