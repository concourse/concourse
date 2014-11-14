package atc

type Resource struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Groups []string `json:"groups"`
	URL    string   `json:"url"`
}
