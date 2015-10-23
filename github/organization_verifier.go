package github

import (
	"net/http"

	"github.com/concourse/atc/auth"
)

type OrganizationVerifier struct {
	organization string
	gitHubClient Client
}

func NewOrganizationVerifier(
	organization string,
	gitHubClient Client,
) auth.Verifier {
	return &OrganizationVerifier{
		organization: organization,
		gitHubClient: gitHubClient,
	}
}

func (verifier *OrganizationVerifier) Verify(httpClient *http.Client) (bool, error) {
	orgs, err := verifier.gitHubClient.Organizations(httpClient)
	if err != nil {
		return false, err
	}

	for _, name := range orgs {
		if name == verifier.organization {
			return true, nil
		}
	}

	return false, nil
}
