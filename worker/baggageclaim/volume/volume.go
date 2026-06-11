package volume

type Volume struct {
	Handle string `json:"handle"`
	Path   string `json:"path"`
	VolumeOpts
}

type Volumes []Volume

type VolumeOpts struct {
	Properties Properties `json:"properties"`
	Privileged bool       `json:"privileged"`
	Uid        *int       `json:"uid,omitempty"`
	Gid        *int       `json:"gid,omitempty"`
}

func (v *VolumeOpts) GetUid() int {
	if v.Uid == nil {
		return -1
	}
	return *v.Uid
}

func (v *VolumeOpts) GetGid() int {
	if v.Gid == nil {
		return -1
	}
	return *v.Gid
}
