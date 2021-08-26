package atc

import (
	"encoding/json"
	"errors"
	"regexp"
)

type Worker struct {
	// not garden_addr, for backwards-compatibility
	GardenAddr      string `json:"addr"`
	BaggageclaimURL string `json:"baggageclaim_url"`

	CertsPath *string `json:"certs_path,omitempty"`

	HTTPProxyURL  string `json:"http_proxy_url,omitempty"`
	HTTPSProxyURL string `json:"https_proxy_url,omitempty"`
	NoProxy       string `json:"no_proxy,omitempty"`

	ActiveContainers int `json:"active_containers"`
	ActiveVolumes    int `json:"active_volumes"`
	ActiveTasks      int `json:"active_tasks"`

	ResourceTypes []WorkerResourceType `json:"resource_types"`

	Platform  string `json:"platform"`
	Tags      Tags   `json:"tags"`
	Team      string `json:"team"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	StartTime int64  `json:"start_time"`
	Ephemeral bool   `json:"ephemeral"`
	State     string `json:"state"`
}

type Tags []string

// UnmarshalJSON unmarshals as a []string, removing any empty elements. Empty
// tags are treated as unset.
func (t *Tags) UnmarshalJSON(data []byte) error {
	var dst []string
	if err := json.Unmarshal(data, &dst); err != nil {
		return err
	}

	if dst == nil {
		return nil
	}

	*t = make(Tags, 0, len(dst))
	for _, s := range dst {
		if s != "" {
			*t = append(*t, s)
		}
	}

	return nil
}

var ErrInvalidWorkerVersion = errors.New("invalid worker version, only numeric characters are allowed")
var ErrMissingWorkerGardenAddress = errors.New("missing garden address")
var ErrNoWorkers = errors.New("no workers available for checking")

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
	Type                 string `json:"type"`
	Image                string `json:"image"`
	Version              string `json:"version"`
	Privileged           bool   `json:"privileged"`
	UniqueVersionHistory bool   `json:"unique_version_history"`
}

type PruneWorkerResponseBody struct {
	Stderr string `json:"stderr"`
}
