package bitbucket

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/skymarshal/verifier"
	"net/http"
)

type RepositoryVerifier struct {
	repositories    []RepositoryConfig
	bitbucketClient Client
}

func NewRepositoryVerifier(repositories []RepositoryConfig, bitbucketClient Client) verifier.Verifier {
	return RepositoryVerifier{
		repositories:    repositories,
		bitbucketClient: bitbucketClient,
	}
}

func (verifier RepositoryVerifier) Verify(logger lager.Logger, httpClient *http.Client) (bool, error) {
	for _, repository := range verifier.repositories {
		accessable, err := verifier.bitbucketClient.Repository(httpClient, repository.OwnerName, repository.RepositoryName)
		if err != nil {
			logger.Error("failed-to-get-repository", err, lager.Data{
				"repository": repository,
			})
			return false, err
		}

		if accessable {
			return true, nil
		}
	}

	logger.Info("not-validated-repositores", lager.Data{
		"want": verifier.repositories,
	})

	return false, nil
}
