package atc

type Worker struct {
	// not garden_addr, for backwards-compatibility
	GardenAddr      string `json:"addr"`
	BaggageclaimURL string `json:"baggageclaim_url"`

	HTTPProxyURL  string `json:"http_proxy_url,omitempty"`
	HTTPSProxyURL string `json:"https_proxy_url,omitempty"`
	NoProxy       string `json:"no_proxy,omitempty"`

	CertificatesPath           string   `json:"certificates_path"`
	CertificatesSymlinkedPaths []string `json:"certificates_symlinked_paths"`

	ActiveContainers int `json:"active_containers"`
	ActiveVolumes    int `json:"active_volumes"`

	ResourceTypes []WorkerResourceType `json:"resource_types"`

	Platform  string   `json:"platform"`
	Tags      []string `json:"tags"`
	Team      string   `json:"team"`
	Name      string   `json:"name"`
	StartTime int64    `json:"start_time"`
	State     string   `json:"state"`
}

type WorkerResourceType struct {
	Type    string `json:"type"`
	Image   string `json:"image"`
	Version string `json:"version"`
}

type PruneWorkerResponseBody struct {
	Stderr string `json:"stderr"`
}
