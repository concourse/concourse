package atc

type Resource struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Groups []string `json:"groups"`
	URL    string   `json:"url"`

	Paused bool `json:"paused,omitempty"`

	FailingToCheck bool   `json:"failing_to_check,omitempty"`
	CheckError     string `json:"check_error,omitempty"`
}
