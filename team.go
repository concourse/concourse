package atc

// Team owns your pipelines
type Team struct {
	// ID is the team's ID
	ID int `json:"id,omitempty"`
	// Name is the team's name
	Name string `json:"name,omitempty"`
}
