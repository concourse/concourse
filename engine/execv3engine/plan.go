package execv3engine

import "github.com/concourse/atc"

type Plan struct {
	Get *GetPlan `json:"get,omitempty"`
}

type GetPlan struct {
	Type         string       `json:"type"`
	Name         string       `json:"name,omitempty"`
	Resource     string       `json:"resource"`
	Source       atc.Source   `json:"source"`
	Params       atc.Params   `json:"params,omitempty"`
	Version      atc.Version  `json:"version,omitempty"`
	Tags         atc.Tags     `json:"tags,omitempty"`
	RootFSSource RootFSSource `json:"rootfs_source,omitempty"`
	Outputs      []string     `json:"outputs,omitempty"`
}

type RootFSSource struct {
	Base   *BaseResourceTypeRootFSSource `json:"base,omitempty"`
	Output *OutputRootFSSource           `json:"output,omitempty"`
}

type BaseResourceTypeRootFSSource struct {
	Name string
}

type OutputRootFSSource struct {
	Name string
}
