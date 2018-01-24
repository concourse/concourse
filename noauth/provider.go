package noauth

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/lager"

	"encoding/json"

	"github.com/concourse/skymarshal/provider"
	"github.com/concourse/skymarshal/verifier"
	flags "github.com/jessevdk/go-flags"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

const ProviderName = "noauth"
const DisplayName = "No Auth"

func init() {
	provider.Register(ProviderName, NoAuthTeamProvider{})
}

type NoAuthConfig struct {
	NoAuth bool `json:"noauth" long:"no-really-i-dont-want-any-auth" description:"Ignore warnings about not configuring auth"`
}

func (config *NoAuthConfig) AuthMethod(oauthBaseURL string, teamName string) provider.AuthMethod {
	return provider.AuthMethod{
		Type:        provider.AuthTypeNone,
		DisplayName: DisplayName,
		AuthURL:     oauthBaseURL + "/auth/basic/token?team_name=" + teamName,
	}
}

func (config *NoAuthConfig) IsConfigured() bool {
	return config.NoAuth
}

func (config *NoAuthConfig) Validate() error {
	return nil
}

func (config *NoAuthConfig) Finalize() error {
	return nil
}

type NoAuthTeamProvider struct{}

func (NoAuthTeamProvider) AddAuthGroup(group *flags.Group) provider.AuthConfig {
	flags := &NoAuthConfig{}

	group, err := group.AddGroup("No Authentication", "", flags)
	if err != nil {
		panic(err)
	}

	return flags
}

func (NoAuthTeamProvider) UnmarshalConfig(config *json.RawMessage) (provider.AuthConfig, error) {
	flags := &NoAuthConfig{}

	if config != nil {
		err := json.Unmarshal(*config, &flags)
		if err != nil {
			return nil, err
		}
	}

	return flags, nil
}

func (NoAuthTeamProvider) MarshalConfig(config provider.AuthConfig) (*json.RawMessage, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	return (*json.RawMessage)(&data), nil
}

func (NoAuthTeamProvider) ProviderConstructor(config provider.AuthConfig, args ...string) (provider.Provider, bool) {

	if c, ok := config.(*NoAuthConfig); ok {
		return Provider{StaticVerifier{c.NoAuth}}, true
	} else {
		return Provider{StaticVerifier{}}, false
	}
}

type StaticVerifier struct {
	Verified bool
}

func (v StaticVerifier) Verify(logger lager.Logger, client *http.Client) (bool, error) {
	return v.Verified, nil
}

type Provider struct {
	verifier.Verifier
}

func (provider Provider) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) (string, error) {
	return "", errors.New("Not supported")
}

func (provider Provider) Exchange(ctx context.Context, req *http.Request) (*oauth2.Token, error) {
	return nil, errors.New("Not supported")
}

func (provider Provider) Client(ctx context.Context, t *oauth2.Token) *http.Client {
	return nil
}

func (p Provider) PreTokenClient() (*http.Client, error) {
	return &http.Client{}, nil
}
