package cloud

import (
	"encoding/json"
	"github.com/concourse/skymarshal/bitbucket"
	"github.com/concourse/skymarshal/provider"
	"github.com/concourse/skymarshal/verifier"
	"github.com/jessevdk/go-flags"
	"golang.org/x/oauth2"
	bitbucketOAuth "golang.org/x/oauth2/bitbucket"
)

const ProviderName = "bitbucket-cloud"
const DisplayName = "Bitbucket Cloud"

var Scopes = []string{"team"}

func init() {
	provider.Register(ProviderName, TeamProvider{})
}

type TeamProvider struct {
}

func (TeamProvider) ProviderConstructor(config provider.AuthConfig, redirectURL string) (provider.Provider, bool) {
	bitbucketAuth := config.(*AuthConfig)

	endpoint := bitbucketOAuth.Endpoint
	if bitbucketAuth.AuthURL != "" && bitbucketAuth.TokenURL != "" {
		endpoint.AuthURL = bitbucketAuth.AuthURL
		endpoint.TokenURL = bitbucketAuth.TokenURL
	}

	teams := make(map[Role][]string)
	for _, team := range bitbucketAuth.Teams {
		teams[team.Role] = append(teams[team.Role], team.Name)
	}

	return Provider{
		Verifier: verifier.NewVerifierBasket(
			bitbucket.NewUserVerifier(bitbucketAuth.Users, NewClient(bitbucketAuth.APIURL)),
			NewTeamVerifier(teams[RoleMember], RoleMember, NewClient(bitbucketAuth.APIURL)),
			NewTeamVerifier(teams[RoleContributor], RoleContributor, NewClient(bitbucketAuth.APIURL)),
			NewTeamVerifier(teams[RoleAdmin], RoleAdmin, NewClient(bitbucketAuth.APIURL)),
			bitbucket.NewRepositoryVerifier(bitbucketAuth.Repositories, NewClient(bitbucketAuth.APIURL)),
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

func (TeamProvider) AddAuthGroup(group *flags.Group) provider.AuthConfig {
	flags := &AuthConfig{}

	bGroup, err := group.AddGroup("Bitbucket Cloud Authentication", "", flags)
	if err != nil {
		panic(err)
	}

	bGroup.Namespace = "bitbucket-cloud-auth"

	return flags
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
