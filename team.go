package atc

import "encoding/json"

type Team struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`

	Auth map[string]*json.RawMessage `json:"auth,omitempty"`
}
