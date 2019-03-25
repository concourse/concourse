package atc

type ClearTaskCacheResponse struct {
	CachesRemoved int64 `json:"caches_removed"`
}

type SaveConfigResponse struct {
	Errors   []string        `json:"errors,omitempty"`
	Warnings []ConfigWarning `json:"warnings,omitempty"`
}

type ConfigResponse struct {
	Config Config `json:"config"`
}
