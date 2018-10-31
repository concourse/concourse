package atc

type Space string

type Metadata []MetadataField

type DefaultSpaceResponse struct {
	DefaultSpace Space `json:"default_space"`
}

type SpaceResponse struct {
	Space Space `json:"space"`
}

type PutResponse struct {
	Space           Space     `json:"space"`
	CreatedVersions []Version `json:"created_versions"`
}

type SpaceVersion struct {
	Space    Space    `json:"space"`
	Version  Version  `json:"version"`
	Metadata Metadata `json:"metadata,omitempty"`
}
