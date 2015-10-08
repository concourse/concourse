package auth

import (
	"net/http"
	"strings"

	"github.com/concourse/atc/github"
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
