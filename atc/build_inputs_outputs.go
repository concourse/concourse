package atc

type BuildInputsOutputs struct {
	Inputs  []PublicBuildInput  `json:"inputs"`
	Outputs []PublicBuildOutput `json:"outputs"`
}

type PublicBuildInput struct {
	Name            string  `json:"name"`
	Version         Version `json:"version"`
	PipelineID      int     `json:"pipeline_id"`
	FirstOccurrence bool    `json:"first_occurrence"`
}

type PublicBuildOutput struct {
	Name    string  `json:"name"`
	Version Version `json:"version"`
}

type ResourceVersion struct {
	ID       int             `json:"id"`
	Metadata []MetadataField `json:"metadata,omitempty"`
	Version  Version         `json:"version"`
	Enabled  bool            `json:"enabled"`
}
