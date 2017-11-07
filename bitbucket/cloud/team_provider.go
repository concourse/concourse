package cloud

import (
	"encoding/json"
	"github.com/concourse/atc/auth/bitbucket"
	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/verifier"
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

	var teamsMember []string
	var teamsContributor []string
	var teamsAdmin []string

	for _, team := range bitbucketAuth.Teams {
		switch bitbucket.Role(team.Role) {
		case bitbucket.RoleMember:
			teamsMember = append(teamsMember, team.TeamName)
		case bitbucket.RoleContributor:
			teamsContributor = append(teamsContributor, team.TeamName)
		case bitbucket.RoleAdmin:
			teamsAdmin = append(teamsAdmin, team.TeamName)
		}
	}

	return Provider{
		Verifier: verifier.NewVerifierBasket(
			bitbucket.NewUserVerifier(bitbucketAuth.Users, NewClient()),
			bitbucket.NewTeamVerifier(teamsMember, bitbucket.RoleMember, NewClient()),
			bitbucket.NewTeamVerifier(teamsContributor, bitbucket.RoleContributor, NewClient()),
			bitbucket.NewTeamVerifier(teamsAdmin, bitbucket.RoleAdmin, NewClient()),
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
