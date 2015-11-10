package atc

type Build struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	JobName      string `json:"job_name,omitempty"`
	URL          string `json:"url"`
	APIURL       string `json:"api_url"`
	PipelineName string `json:"pipeline_name,omitempty"`
	StartTime    int64  `json:"start_time,omitempty"`
	EndTime      int64  `json:"end_time,omitempty"`
}

type BuildStatus string

const (
	StatusStarted   BuildStatus = "started"
	StatusSucceeded BuildStatus = "succeeded"
	StatusFailed    BuildStatus = "failed"
	StatusErrored   BuildStatus = "errored"
	StatusAborted   BuildStatus = "aborted"
)
