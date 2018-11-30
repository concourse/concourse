package atc

type Space string

type Metadata []MetadataField

type DefaultSpaceResponse struct {
	DefaultSpace Space `json:"default_space"`
}

type SpaceResponse struct {
	Space Space `json:"space"`
}

type SpaceVersion struct {
	Space    Space    `json:"space"`
	Version  Version  `json:"version"`
	Metadata Metadata `json:"metadata,omitempty"`
}
