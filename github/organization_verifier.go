package github

import (
	"net/http"

	"github.com/concourse/atc/auth"
)

type OrganizationVerifier struct {
	organizations []string
	gitHubClient  Client
}

func NewOrganizationVerifier(
	organizations []string,
	gitHubClient Client,
) auth.Verifier {
	return &OrganizationVerifier{
		organizations: organizations,
		gitHubClient:  gitHubClient,
	}
}

func (verifier *OrganizationVerifier) Verify(httpClient *http.Client) (bool, error) {
	orgs, err := verifier.gitHubClient.Organizations(httpClient)
	if err != nil {
		return false, err
	}

	for _, name := range orgs {
		for _, authorizedOrg := range verifier.organizations {
			if name == authorizedOrg {
				return true, nil
			}
		}
	}

	return false, nil
}
