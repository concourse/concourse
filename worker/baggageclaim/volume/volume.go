package volume

type Volume struct {
	Handle     string     `json:"handle"`
	Path       string     `json:"path"`
	Properties Properties `json:"properties"`
	Privileged bool       `json:"privileged"`
	Size       int        `json:"size"`
}

type Volumes []Volume
