package skycmd

type OIDCFlags struct {
	Issuer        string   `long:"issuer" description:"Issuer URL"`
	ClientID      string   `long:"client-id" description:"Client id"`
	ClientSecret  string   `long:"client-secret" description:"Client secret"`
	Scopes        []string `long:"scope" description:"Requested scope"`
	HostedDomains []string `long:"hosted-domain" description:"Hosted domain"`
}

func (self OIDCFlags) IsValid() bool {
	return self.Issuer != "" && self.ClientID != "" && self.ClientSecret != ""
}

type OIDCTeamFlags struct {
	Users  []string `json:"users" long:"user" description:"List of OIDC users" value-name:"OIDC_USERNAME"`
	Groups []string `json:"groups" long:"group" description:"List of OIDC groups" value-name:"OIDC_GROUP"`
}

func (self OIDCTeamFlags) IsValid() bool {
	return len(self.Users) > 0 || len(self.Groups) > 0
}
