package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/concourse/atc/github"
	"github.com/concourse/atc/web/routes"
	"github.com/tedsuo/rata"
)

type GitHubOrganizationValidator struct {
	Organization string
	Client       github.Client
}

func (validator GitHubOrganizationValidator) IsAuthenticated(r *http.Request) bool {
	authorizationHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authorizationHeader, "Token ") {
		return false
	}
	accessToken := authorizationHeader[6:]

	orgs, err := validator.Client.GetOrganizations(accessToken)
	if err != nil {
		return false
	}

	for _, org := range orgs {
		if org == validator.Organization {
			return true
		}
	}

	return false
}

func (validator GitHubOrganizationValidator) Unauthorized(w http.ResponseWriter, r *http.Request) {
	path, err := routes.Routes.CreatePathForRoute(routes.LogIn, rata.Params{})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "internal error, whoops")
	}
	http.Redirect(w, r, path, http.StatusFound)
}
