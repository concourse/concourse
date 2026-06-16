package baggageclaim

import (
	"encoding/json"
)

type VolumeRequest struct {
	Handle     string           `json:"handle"`
	Strategy   *json.RawMessage `json:"strategy"`
	Properties VolumeProperties `json:"properties"`
	Privileged bool             `json:"privileged,omitempty"`

	// The Uid and Gid that the volume should be owned by inside the container's
	// user namespace. A value of '-1' means to leave ownership as-is. Both
	// fields must be set to a value >= 0 if you want ownership to be changed.
	Uid *int `json:"uid,omitempty"`
	Gid *int `json:"gid,omitempty"`
}

func (v *VolumeRequest) GetUid() int {
	if v.Uid == nil {
		return -1
	}
	return *v.Uid
}

func (v *VolumeRequest) GetGid() int {
	if v.Gid == nil {
		return -1
	}
	return *v.Gid
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
