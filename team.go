package atc

import "encoding/json"

type Team struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`

	BasicAuth *BasicAuth `json:"basic_auth,omitempty"`

	Auth map[string]*json.RawMessage `json:"auth,omitempty"`
}

type BasicAuth struct {
	BasicAuthUsername string `json:"basic_auth_username,omitempty"`
	BasicAuthPassword string `json:"basic_auth_password,omitempty"`
}
