package atc

type BuildInputsOutputs struct {
	Inputs  []VersionedResource `json:"inputs"`
	Outputs []VersionedResource `json:"outputs"`
}

type VersionedResource struct {
	Resource string  `json:"resource"`
	Version  Version `json:"version"`
}
