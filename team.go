package atc

// Team owns your pipelines
type Team struct {
	// ID is the team's ID
	ID int `json:"id,omitempty"`

	// Name is the team's name
	Name string `json:"name,omitempty"`

	BasicAuth    *BasicAuth    `json:"basic_auth,omitempty"`
	GitHubAuth   *GitHubAuth   `json:"github_auth,omitempty"`
	UAAAuth      *UAAAuth      `json:"uaa_auth,omitempty"`
	GenericOAuth *GenericOAuth `json:"genericoauth_auth,omitempty"`
}

type BasicAuth struct {
	BasicAuthUsername string `json:"basic_auth_username,omitempty"`
	BasicAuthPassword string `json:"basic_auth_password,omitempty"`
}

type GitHubAuth struct {
	ClientID      string       `json:"client_id,omitempty"`
	ClientSecret  string       `json:"client_secret,omitempty"`
	Organizations []string     `json:"organizations,omitempty"`
	Teams         []GitHubTeam `json:"teams,omitempty"`
	Users         []string     `json:"users,omitempty"`
	AuthURL       string       `json:"auth_url,omitempty"`
	TokenURL      string       `json:"token_url,omitempty"`
	APIURL        string       `json:"api_url,omitempty"`
}

type GitHubTeam struct {
	OrganizationName string `json:"organization_name,omitempty"`
	TeamName         string `json:"team_name,omitempty"`
}

type UAAAuth struct {
	ClientID     string   `json:"client_id,omitempty"`
	ClientSecret string   `json:"client_secret,omitempty"`
	AuthURL      string   `json:"auth_url,omitempty"`
	TokenURL     string   `json:"token_url,omitempty"`
	CFSpaces     []string `json:"cf_spaces,omitempty"`
	CFURL        string   `json:"cf_url,omitempty"`
	CFCACert     string   `json:"cf_ca_cert,omitempty"`
}

type GenericOAuth struct {
	DisplayName   string            `json:"display_name,omitempty"`
	ClientID      string            `json:"client_id,omitempty"`
	ClientSecret  string            `json:"client_secret,omitempty"`
	AuthURL       string            `json:"auth_url,omitempty"`
	TokenURL      string            `json:"token_url,omitempty"`
	AuthURLParams map[string]string `json:"auth_url_params,omitempty"`
}
