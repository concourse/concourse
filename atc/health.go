package atc

type Health struct {
	DBStatus   string `json:"db_status,omitempty"`
	WorkersStatus   map[string] string `json:"workers_status,omitempty"`
}