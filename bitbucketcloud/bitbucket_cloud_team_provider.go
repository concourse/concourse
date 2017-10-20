package bitbucketcloud

import (
	"encoding/json"
	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/verifier"
	"github.com/jessevdk/go-flags"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/bitbucket"
)

const ProviderName = "bitbucket-cloud"
const DisplayName = "Bitbucket Cloud"

var Scopes = []string{"team"}

func init() {
	provider.Register(ProviderName, BitbucketCloudTeamProvider{})
}

type BitbucketCloudTeamProvider struct {
}

func (BitbucketCloudTeamProvider) ProviderConstructor(config provider.AuthConfig, redirectURL string) (provider.Provider, bool) {
	bitbucketAuth := config.(*BitbucketCloudAuthConfig)

	endpoint := bitbucket.Endpoint
	if bitbucketAuth.AuthURL != "" && bitbucketAuth.TokenURL != "" {
		endpoint.AuthURL = bitbucketAuth.AuthURL
		endpoint.TokenURL = bitbucketAuth.TokenURL
	}

	return BitbucketCloudProvider{
		Verifier: verifier.NewVerifierBasket(
			NewUserVerifier(bitbucketAuth.Users),
		),
		Config: &oauth2.Config{
			ClientID:     bitbucketAuth.ClientID,
			ClientSecret: bitbucketAuth.ClientSecret,
			Endpoint:     endpoint,
			Scopes:       Scopes,
			RedirectURL:  redirectURL,
		},
	}, true
}

func (BitbucketCloudTeamProvider) AddAuthGroup(group *flags.Group) provider.AuthConfig {
	flags := &BitbucketCloudAuthConfig{}

	bGroup, err := group.AddGroup("Bitbucket Cloud Authentication", "", flags)
	if err != nil {
		panic(err)
	}

	bGroup.Namespace = "bitbucket-cloud-auth"

	return flags
}

func (BitbucketCloudTeamProvider) UnmarshalConfig(config *json.RawMessage) (provider.AuthConfig, error) {
	flags := &BitbucketCloudAuthConfig{}
	if config != nil {
		err := json.Unmarshal(*config, &flags)
		if err != nil {
			return nil, err
		}
	}
	return flags, nil
}
