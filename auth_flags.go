package atc

import (
	"errors"

	multierror "github.com/hashicorp/go-multierror"
)

type BasicAuthFlag struct {
	Username string `long:"username" description:"Username to use for basic auth."`
	Password string `long:"password" description:"Password to use for basic auth."`
}

func (auth *BasicAuthFlag) IsConfigured() bool {
	return auth.Username != "" || auth.Password != ""
}

func (auth *BasicAuthFlag) Validate() error {
	var errs *multierror.Error
	if auth.Username == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --basic-auth-username to use basic auth."),
		)
	}
	if auth.Password == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --basic-auth-password to use basic auth."),
		)
	}
	return errs.ErrorOrNil()
}

type GitHubAuthFlag struct {
	ClientID      string           `long:"client-id"     description:"Application client ID for enabling GitHub OAuth."`
	ClientSecret  string           `long:"client-secret" description:"Application client secret for enabling GitHub OAuth."`
	Organizations []string         `long:"organization"  description:"GitHub organization whose members will have access." value-name:"ORG"`
	Teams         []GitHubTeamFlag `long:"team"          description:"GitHub team whose members will have access." value-name:"ORG/TEAM"`
	Users         []string         `long:"user"          description:"GitHub user to permit access." value-name:"LOGIN"`
	AuthURL       string           `long:"auth-url"      description:"Override default endpoint AuthURL for Github Enterprise."`
	TokenURL      string           `long:"token-url"     description:"Override default endpoint TokenURL for Github Enterprise."`
	APIURL        string           `long:"api-url"       description:"Override default API endpoint URL for Github Enterprise."`
}

func (auth *GitHubAuthFlag) IsConfigured() bool {
	return auth.ClientID != "" ||
		auth.ClientSecret != "" ||
		len(auth.Organizations) > 0 ||
		len(auth.Teams) > 0 ||
		len(auth.Users) > 0
}

func (auth *GitHubAuthFlag) Validate() error {
	var errs *multierror.Error
	if auth.ClientID == "" || auth.ClientSecret == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --github-auth-client-id and --github-auth-client-secret to use GitHub OAuth."),
		)
	}
	if len(auth.Organizations) == 0 && len(auth.Teams) == 0 && len(auth.Users) == 0 {
		errs = multierror.Append(
			errs,
			errors.New("at least one of the following is required for github-auth: organizations, teams, users."),
		)
	}
	return errs.ErrorOrNil()
}

type GenericOAuthFlag struct {
	DisplayName   string            `long:"display-name"   description:"Name for this auth method on the web UI."`
	ClientID      string            `long:"client-id"      description:"Application client ID for enabling generic OAuth."`
	ClientSecret  string            `long:"client-secret"  description:"Application client secret for enabling generic OAuth."`
	AuthURL       string            `long:"auth-url"       description:"Generic OAuth provider AuthURL endpoint."`
	AuthURLParams map[string]string `long:"auth-url-param" description:"Parameter to pass to the authentication server AuthURL. Can be specified multiple times."`
	TokenURL      string            `long:"token-url"      description:"Generic OAuth provider TokenURL endpoint."`
}

func (auth *GenericOAuthFlag) IsConfigured() bool {
	return auth.AuthURL != "" ||
		auth.TokenURL != "" ||
		auth.ClientID != "" ||
		auth.ClientSecret != "" ||
		auth.DisplayName != ""
}

func (auth *GenericOAuthFlag) Validate() error {
	var errs *multierror.Error
	if auth.ClientID == "" || auth.ClientSecret == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --generic-oauth-client-id and --generic-oauth-client-secret to use Generic OAuth."),
		)
	}
	if auth.AuthURL == "" || auth.TokenURL == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --generic-oauth-auth-url and --generic-oauth-token-url to use Generic OAuth."),
		)
	}
	if auth.DisplayName == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --generic-oauth-display-name to use Generic OAuth."),
		)
	}
	return errs.ErrorOrNil()
}

type UAAAuthFlag struct {
	ClientID     string   `long:"client-id"     description:"Application client ID for enabling UAA OAuth."`
	ClientSecret string   `long:"client-secret" description:"Application client secret for enabling UAA OAuth."`
	AuthURL      string   `long:"auth-url"      description:"UAA AuthURL endpoint."`
	TokenURL     string   `long:"token-url"     description:"UAA TokenURL endpoint."`
	CFSpaces     []string `long:"cf-space"      description:"Space GUID for a CF space whose developers will have access."`
	CFURL        string   `long:"cf-url"        description:"CF API endpoint."`
	CFCACert     PathFlag `long:"cf-ca-cert"    description:"Path to CF PEM-encoded CA certificate file."`
}

func (auth *UAAAuthFlag) IsConfigured() bool {
	return auth.ClientID != "" ||
		auth.ClientSecret != "" ||
		len(auth.CFSpaces) > 0 ||
		auth.AuthURL != "" ||
		auth.TokenURL != "" ||
		auth.CFURL != ""
}

func (auth *UAAAuthFlag) Validate() error {
	var errs *multierror.Error
	if auth.ClientID == "" || auth.ClientSecret == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --uaa-auth-client-id and --uaa-auth-client-secret to use UAA OAuth."),
		)
	}
	if len(auth.CFSpaces) == 0 {
		errs = multierror.Append(
			errs,
			errors.New("must specify --uaa-auth-cf-space to use UAA OAuth."),
		)
	}
	if auth.AuthURL == "" || auth.TokenURL == "" || auth.CFURL == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --uaa-auth-auth-url, --uaa-auth-token-url and --uaa-auth-cf-url to use UAA OAuth."),
		)
	}
	return errs.ErrorOrNil()
}
