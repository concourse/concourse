package atc

type Pipeline struct {
	ID          int            `json:"id"`
	Name        string         `json:"name"`
	Paused      bool           `json:"paused"`
	Public      bool           `json:"public"`
	Archived    bool           `json:"archived"`
	Groups      GroupConfigs   `json:"groups,omitempty"`
	TeamName    string         `json:"team_name"`
	Display     *DisplayConfig `json:"display,omitempty"`
	LastUpdated int64          `json:"last_updated,omitempty"`
}

type RenameRequest struct {
	NewName string `json:"name"`
}
