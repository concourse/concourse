package atc

type BuildInputsOutputs struct {
	Inputs  []PublicBuildInput  `json:"inputs"`
	Outputs []VersionedResource `json:"outputs"`
}

type PublicBuildInput struct {
	Name            string          `json:"name"`
	Resource        string          `json:"resource"`
	Type            string          `json:"type"`
	Version         Version         `json:"version"`
	Metadata        []MetadataField `json:"metadata"`
	PipelineName    string          `json:"pipeline_name"`
	FirstOccurrence bool            `json:"first_occurrence"`
}

type VersionedResource struct {
	ID           int             `json:"id"`
	PipelineName string          `json:"pipeline_name"`
	Type         string          `json:"type"`
	Metadata     []MetadataField `json:"metadata"`
	Resource     string          `json:"resource"`
	Version      Version         `json:"version"`
	Enabled      bool            `json:"enabled"`
}
