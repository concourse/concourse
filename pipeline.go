package atc

type Pipeline struct {
	Name     string       `json:"name"`
	URL      string       `json:"url"`
	Paused   bool         `json:"paused"`
	Groups   GroupConfigs `json:"groups,omitempty"`
	TeamName string       `json:"team_name"`
}
