package atc

type Pipe struct {
	ID string `json:"id"`

	ReadURL  string `json:"read_url"`
	WriteURL string `json:"write_url"`
}
