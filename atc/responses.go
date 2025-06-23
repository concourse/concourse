package atc

type ClearTaskCacheResponse struct {
	CachesRemoved int64 `json:"caches_removed"`
}

type SaveConfigResponse struct {
	Errors       []string       `json:"errors,omitempty"`
	ConfigErrors []ConfigErrors `json:"configerrors,omitempty"`
}

type ConfigResponse struct {
	Config Config `json:"config"`
}

type ClearResourceCacheResponse struct {
	CachesRemoved int64 `json:"caches_removed"`
}

type ClearVersionsResponse struct {
	VersionsRemoved int64 `json:"versions_removed"`
}
