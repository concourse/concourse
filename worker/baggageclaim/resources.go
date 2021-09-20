package baggageclaim

import (
	"encoding/json"
)

type VolumeRequest struct {
	Handle     string           `json:"handle"`
	Strategy   *json.RawMessage `json:"strategy"`
	Properties VolumeProperties `json:"properties"`
	Privileged bool             `json:"privileged,omitempty"`
}

type VolumeResponse struct {
	Handle     string           `json:"handle"`
	Path       string           `json:"path"`
	Properties VolumeProperties `json:"properties"`
}

type VolumeFutureResponse struct {
	Handle string `json:"handle"`
}

type PropertyRequest struct {
	Value string `json:"value"`
}

type PrivilegedRequest struct {
	Value bool `json:"value"`
}
