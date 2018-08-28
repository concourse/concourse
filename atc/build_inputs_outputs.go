package atc

type BuildInputsOutputs struct {
	Inputs  []PublicBuildInput  `json:"inputs"`
	Outputs []VersionedResource `json:"outputs"`
}

type PublicBuildInput struct {
	Name            string  `json:"name"`
	Version         Version `json:"version"`
	PipelineID      int     `json:"pipeline_id"`
	FirstOccurrence bool    `json:"first_occurrence"`
}

type VersionedResource struct {
	ID       int             `json:"id"`
	Type     string          `json:"type"`
	Metadata []MetadataField `json:"metadata"`
	Resource string          `json:"resource"`
	Version  Version         `json:"version"`
	Enabled  bool            `json:"enabled"`
}
