package atc

type Build struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	JobName string `json:"job_name"`
	URL     string `json:"url"`
}
