package basicauth

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/lager"

	"encoding/json"

	"github.com/concourse/skymarshal/provider"
	"github.com/concourse/skymarshal/verifier"
	multierror "github.com/hashicorp/go-multierror"
	flags "github.com/jessevdk/go-flags"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

const ProviderName = "basicauth"
const DisplayName = "Basic Auth"

func init() {
	provider.Register(ProviderName, BasicAuthTeamProvider{})
}

type BasicAuthConfig struct {
	Username string `json:"username" long:"username" description:"Username to use for basic auth."`
	Password string `json:"password" long:"password" description:"Password to use for basic auth."`
}

func (config *BasicAuthConfig) AuthMethod(oauthBaseURL string, teamName string) provider.AuthMethod {
	return provider.AuthMethod{
		Type:        provider.AuthTypeBasic,
		DisplayName: DisplayName,
		AuthURL:     oauthBaseURL + "/auth/basic/token?team_name=" + teamName,
	}
}

func (config *BasicAuthConfig) IsConfigured() bool {
	return config.Username != "" || config.Password != ""
}

func (config *BasicAuthConfig) Validate() error {
	var errs *multierror.Error
	if config.Username == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --basic-auth-username to use basic auth."),
		)
	}
	if config.Password == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --basic-auth-password to use basic auth."),
		)
	}
	return errs.ErrorOrNil()
}

func (config *BasicAuthConfig) Finalize() error {
	if cost, err := bcrypt.Cost([]byte(config.Password)); err == nil && cost > 0 {
		// This password has already been hashed so nothing to see here
		return nil
	}

	if encrypted, err := bcrypt.GenerateFromPassword([]byte(config.Password), bcrypt.MinCost); err != nil {
		return err
	} else {
		config.Password = string(encrypted)
		return nil
	}
}

type BasicAuthTeamProvider struct{}

func (BasicAuthTeamProvider) AddAuthGroup(group *flags.Group) provider.AuthConfig {
	flags := &BasicAuthConfig{}

	group, err := group.AddGroup("Basic Authentication", "", flags)
	if err != nil {
		panic(err)
	}
	group.Namespace = "basic-auth"

	return flags
}

func (BasicAuthTeamProvider) UnmarshalConfig(config *json.RawMessage) (provider.AuthConfig, error) {
	flags := &BasicAuthConfig{}

	if config != nil {
		err := json.Unmarshal(*config, &flags)
		if err != nil {
			return nil, err
		}
	}
	return flags, nil
}

func (BasicAuthTeamProvider) MarshalConfig(config provider.AuthConfig) (*json.RawMessage, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	return (*json.RawMessage)(&data), nil
}

func (BasicAuthTeamProvider) ProviderConstructor(config provider.AuthConfig, args ...string) (provider.Provider, bool) {

	if c, ok := config.(*BasicAuthConfig); ok && len(args) == 2 {
		return Provider{BasicVerifier{c, args[0], args[1]}}, true
	} else {
		return Provider{BasicVerifier{}}, false
	}
}

type BasicVerifier struct {
	config   *BasicAuthConfig
	Username string
	Password string
}

func (v BasicVerifier) Verify(logger lager.Logger, client *http.Client) (bool, error) {

	if v.config == nil {
		return false, errors.New("No config information")
	}

	if v.config.Username != v.Username {
		return false, errors.New("Incorrect username")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(v.config.Password), []byte(v.Password)); err != nil {
		return false, errors.New("Incorrect password")
	}

	return true, nil
}

type Provider struct {
	verifier.Verifier
}

func (p Provider) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) (string, error) {
	return "", errors.New("Not supported")
}

func (p Provider) Exchange(ctx context.Context, req *http.Request) (*oauth2.Token, error) {
	return nil, errors.New("Not supported")
}

func (p Provider) Client(ctx context.Context, t *oauth2.Token) *http.Client {
	return nil
}

func (p Provider) PreTokenClient() (*http.Client, error) {
	return nil, errors.New("Not supported")
}
