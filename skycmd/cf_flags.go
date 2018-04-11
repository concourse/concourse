package skycmd

type CFFlags struct {
	ClientID           string   `long:"client-id" description:"Client id"`
	ClientSecret       string   `long:"client-secret" description:"Client secret"`
	APIURL             string   `long:"api-url" description:"API URL"`
	RootCAs            []string `long:"root-ca" description:"Root CA"`
	InsecureSkipVerify bool     `long:"skip-ssl-validation" description:"Skip SSL validation"`
}

func (self CFFlags) IsValid() bool {
	return self.ClientID != "" && self.ClientSecret != ""
}

type CFTeamFlags struct {
	Groups []string `json:"groups" long:"group" description:"List of cf groups (e.g. my-org or my-org:my-space)" value-name:"CF_ORG:CF_SPACE"`
}

func (config CFTeamFlags) IsValid() bool {
	return len(config.Groups) > 0
}
