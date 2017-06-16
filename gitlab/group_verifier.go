package gitlab

import (
	"net/http"

	"code.cloudfoundry.org/lager"
)

type GroupVerifier struct {
	groups       []string
	gitLabClient Client
}

func NewGroupVerifier(
	groups []string,
	gitLabClient Client,
) GroupVerifier {
	return GroupVerifier{
		groups:       groups,
		gitLabClient: gitLabClient,
	}
}

func (verifier GroupVerifier) Verify(logger lager.Logger, httpClient *http.Client) (bool, error) {
	groups, err := verifier.gitLabClient.Groups(httpClient)
	if err != nil {
		logger.Error("failed-to-get-groups", err)
		return false, err
	}

	for _, name := range groups {
		for _, authorizedGroup := range verifier.groups {
			if name == authorizedGroup {
				return true, nil
			}
		}
	}

	logger.Info("not-in-groups", lager.Data{
		"have": groups,
		"want": verifier.groups,
	})

	return false, nil
}
