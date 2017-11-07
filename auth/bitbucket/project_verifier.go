package bitbucket

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/auth/verifier"
	"net/http"
)

type ProjectVerifier struct {
	projects        []string
	bitbucketClient Client
}

func NewProjectVerifier(projects []string, bitbucketClient Client) verifier.Verifier {
	return ProjectVerifier{
		projects:        projects,
		bitbucketClient: bitbucketClient,
	}
}

func (verifier ProjectVerifier) Verify(logger lager.Logger, httpClient *http.Client) (bool, error) {
	projects, err := verifier.bitbucketClient.Projects(httpClient)
	if err != nil {
		logger.Error("failed-to-get-projects", err)
		return false, err
	}

	for _, project := range projects {
		for _, verifierProject := range verifier.projects {
			if project == verifierProject {
				return true, nil
			}
		}
	}

	logger.Info("not-validated-projects", lager.Data{
		"have": projects,
		"want": verifier.projects,
	})

	return false, nil
}
