package atc

type Worker struct {
	// not garden_addr, for backwards-compatibility
	GardenAddr string `json:"addr"`

	BaggageclaimURL string `json:"baggageclaim_url"`

	ActiveContainers int `json:"active_containers"`

	ResourceTypes []WorkerResourceType `json:"resource_types"`

	Platform string   `json:"platform"`
	Tags     []string `json:"tags"`
}

type WorkerResourceType struct {
	Type  string `json:"type"`
	Image string `json:"image"`
}
