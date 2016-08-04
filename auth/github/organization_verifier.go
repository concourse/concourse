package github

import (
	"net/http"

	"code.cloudfoundry.org/lager"
)

type OrganizationVerifier struct {
	organizations []string
	gitHubClient  Client
}

func NewOrganizationVerifier(
	organizations []string,
	gitHubClient Client,
) OrganizationVerifier {
	return OrganizationVerifier{
		organizations: organizations,
		gitHubClient:  gitHubClient,
	}
}

func (verifier OrganizationVerifier) Verify(logger lager.Logger, httpClient *http.Client) (bool, error) {
	orgs, err := verifier.gitHubClient.Organizations(httpClient)
	if err != nil {
		logger.Error("failed-to-get-organizations", err)
		return false, err
	}

	for _, name := range orgs {
		for _, authorizedOrg := range verifier.organizations {
			if name == authorizedOrg {
				return true, nil
			}
		}
	}

	logger.Info("not-in-organizations", lager.Data{
		"have": orgs,
		"want": verifier.organizations,
	})

	return false, nil
}
