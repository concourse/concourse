package server

import (
	"encoding/json"
	"strings"

	"github.com/concourse/skymarshal/bitbucket"
	"github.com/concourse/skymarshal/provider"
	"github.com/concourse/skymarshal/verifier"
	"github.com/dghubble/oauth1"
	"github.com/jessevdk/go-flags"
)

const ProviderName = "bitbucket-server"
const DisplayName = "Bitbucket Server"

var Scopes = []string{"team"}

func init() {
	provider.Register(ProviderName, TeamProvider{})
}

type TeamProvider struct {
}

func (TeamProvider) AddAuthGroup(group *flags.Group) provider.AuthConfig {
	flags := &AuthConfig{}

	bGroup, err := group.AddGroup("Bitbucket Server Authentication", "", flags)
	if err != nil {
		panic(err)
	}

	bGroup.Namespace = "bitbucket-server-auth"

	return flags
}

func (TeamProvider) ProviderConstructor(config provider.AuthConfig, redirectURL string) (provider.Provider, bool) {
	bitbucketAuth := config.(*AuthConfig)

	endpoint := oauth1.Endpoint{
		RequestTokenURL: strings.TrimRight(bitbucketAuth.Endpoint, "/") + "/plugins/servlet/oauth/request-token",
		AuthorizeURL:    strings.TrimRight(bitbucketAuth.Endpoint, "/") + "/plugins/servlet/oauth/authorize",
		AccessTokenURL:  strings.TrimRight(bitbucketAuth.Endpoint, "/") + "/plugins/servlet/oauth/access-token",
	}

	var projects []string
	for _, project := range bitbucketAuth.Projects {
		projects = append(projects, project)
	}

	return &Provider{
		Verifier: verifier.NewVerifierBasket(
			bitbucket.NewUserVerifier(bitbucketAuth.Users, &client{bitbucketAuth.Endpoint}),
			NewProjectVerifier(projects, &client{bitbucketAuth.Endpoint}),
			bitbucket.NewRepositoryVerifier(bitbucketAuth.Repositories, &client{bitbucketAuth.Endpoint}),
		),
		Config: &oauth1.Config{
			ConsumerKey: bitbucketAuth.ConsumerKey,
			CallbackURL: redirectURL,
			Endpoint:    endpoint,
			Signer: &oauth1.RSASigner{
				PrivateKey: bitbucketAuth.PrivateKey.PrivateKey,
			},
		},
		secrets: make(map[string]string),
	}, true
}

func (TeamProvider) UnmarshalConfig(config *json.RawMessage) (provider.AuthConfig, error) {
	flags := &AuthConfig{}
	if config != nil {
		err := json.Unmarshal(*config, &flags)
		if err != nil {
			return nil, err
		}
	}
	return flags, nil
}
