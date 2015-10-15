package auth

import (
	"net/http"

	"github.com/google/go-github/github"
)

type GitHubOrganizationVerifier struct {
	Organization string
}

func (verifier *GitHubOrganizationVerifier) Verify(httpClient *http.Client) (bool, error) {
	gitHubClient := github.NewClient(httpClient)

	orgs, _, err := gitHubClient.Organizations.List("", nil)
	if err != nil {
		return false, err
	}

	for _, o := range orgs {
		if *o.Login == verifier.Organization {
			return true, nil
		}
	}

	return false, nil
}
