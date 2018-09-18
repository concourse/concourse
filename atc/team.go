package atc

type Team struct {
	ID   int                 `json:"id,omitempty"`
	Name string              `json:"name,omitempty"`
	Auth map[string][]string `json:"auth,omitempty"`
}
