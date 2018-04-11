package skycmd

type LocalTeamFlags struct {
	Users []string `json:"users" long:"user" description:"List of basic auth users" value-name:"BASIC_AUTH_USERNAME"`
}

func (config LocalTeamFlags) IsValid() bool {
	return len(config.Users) > 0
}
