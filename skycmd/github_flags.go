package skycmd

type GithubFlags struct {
	ClientID     string `long:"client-id" description:"Client id"`
	ClientSecret string `long:"client-secret" description:"Client secret"`
}

func (self GithubFlags) IsValid() bool {
	return self.ClientID != "" && self.ClientSecret != ""
}

type GithubTeamFlags struct {
	Users  []string `json:"users" long:"user" description:"List of github users" value-name:"GITHUB_LOGIN"`
	Groups []string `json:"groups" long:"group" description:"List of github groups (e.g. my-org or my-org:my-team)" value-name:"GITHUB_ORG:GITHUB_TEAM"`
}

func (config GithubTeamFlags) IsValid() bool {
	return len(config.Users) > 0 || len(config.Groups) > 0
}
