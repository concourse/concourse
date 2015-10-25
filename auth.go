package atc

type AuthType string

const (
	AuthTypeBasic AuthType = "basic"
	AuthTypeOAuth AuthType = "oauth"
)

type AuthMethod struct {
	Type AuthType `json:"type"`

	DisplayName string `json:"display_name"`
	AuthURL     string `json:"auth_url"`
}

type AuthToken struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
