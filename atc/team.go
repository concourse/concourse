package atc

type Team struct {
	ID   int      `json:"id,omitempty"`
	Name string   `json:"name,omitempty"`
	Auth TeamAuth `json:"auth,omitempty"`
}

type TeamAuth map[string]map[string][]string
