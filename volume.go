package atc

import "encoding/json"

type Volume struct {
	ID              string           `json:"id"`
	TTLInSeconds    int64            `json:"ttl_in_seconds"`
	ResourceVersion *json.RawMessage `json:"resource_version"`
	WorkerName      string           `json:"worker_name"`
}
