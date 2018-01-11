package server

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/skymarshal/bitbucket"
	"github.com/concourse/skymarshal/verifier"
	"net/http"
)

type ProjectVerifier struct {
	projects        []string
	bitbucketClient bitbucket.Client
}

func NewProjectVerifier(projects []string, bitbucketClient bitbucket.Client) verifier.Verifier {
	return ProjectVerifier{
		projects:        projects,
		bitbucketClient: bitbucketClient,
	}
}

func (verifier ProjectVerifier) Verify(logger lager.Logger, httpClient *http.Client) (bool, error) {
	if len(verifier.projects) == 0 {
		return false, nil
	}

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
