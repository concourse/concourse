package atc

type VersionedResource struct {
	Resource string  `json:"resource"`
	Version  Version `json:"version"`
}
