package atc

type Claims struct {
	Sub               string `json:"sub,omitempty"`
	UserID            string `json:"userid,omitempty"`
	UserName          string `json:"username,omitempty"`
	PreferredUsername string `json:"preferred_username,omitempty"`
	Email             string `json:"email,omitempty"`
	Connector         string `json:"connector,omitempty"`
}
