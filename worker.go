package atc

type Worker struct {
	// not garden_addr, for backwards-compatibility
	GardenAddr      string `json:"addr"`
	BaggageclaimURL string `json:"baggageclaim_url"`

	HTTPProxyURL  string `json:"http_proxy_url,omitempty"`
	HTTPSProxyURL string `json:"https_proxy_url,omitempty"`
	NoProxy       string `json:"no_proxy,omitempty"`

	ActiveContainers int `json:"active_containers"`

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
