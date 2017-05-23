package atc

import (
	"errors"
	"regexp"
)

type Worker struct {
	// not garden_addr, for backwards-compatibility
	GardenAddr      string `json:"addr"`
	BaggageclaimURL string `json:"baggageclaim_url"`

	HTTPProxyURL  string `json:"http_proxy_url,omitempty"`
	HTTPSProxyURL string `json:"https_proxy_url,omitempty"`
	NoProxy       string `json:"no_proxy,omitempty"`

	ActiveContainers int `json:"active_containers"`
	ActiveVolumes    int `json:"active_volumes"`

	ResourceTypes []WorkerResourceType `json:"resource_types"`

	Platform  string   `json:"platform"`
	Tags      []string `json:"tags"`
	Team      string   `json:"team"`
	Name      string   `json:"name"`
	Version   string   `json:"version"`
	StartTime int64    `json:"start_time"`
	State     string   `json:"state"`
}

var ErrInvalidWorkerVersion = errors.New("invalid worker version, only numeric characters are allowed")
var ErrMissingWorkerGardenAddress = errors.New("missing garden address")

func (w Worker) Validate() error {
	if w.Version != "" && !regexp.MustCompile(`^[0-9\.]+$`).MatchString(w.Version) {
		return ErrInvalidWorkerVersion
	}

	if len(w.GardenAddr) == 0 {
		return ErrMissingWorkerGardenAddress
	}

	return nil
}

type WorkerResourceType struct {
	Type       string `json:"type"`
	Image      string `json:"image"`
	Version    string `json:"version"`
	Privileged bool   `json:"privileged"`
}

type PruneWorkerResponseBody struct {
	Stderr string `json:"stderr"`
}
