package atc

// Team owns your pipelines
type Team struct {
	// ID is the team's ID
	ID int `json:"id,omitempty"`

	// Name is the team's name
	Name string `json:"name,omitempty"`

	BasicAuth
	GitHubAuth
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
	AuthURL       string       `json:"authurl,omitempty"`
	TokenURL      string       `json:"tokenurl,omitempty"`
	APIURL        string       `json:"apiurl,omitempty"`
}

type GitHubTeam struct {
	OrganizationName string `json:"organization_name,omitempty"`
	TeamName         string `json:"team_name,omitempty"`
}
