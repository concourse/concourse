package atc

import "encoding/json"

type PublicBuildPlan struct {
	Schema string           `json:"schema"`
	Plan   *json.RawMessage `json:"plan"`
}
