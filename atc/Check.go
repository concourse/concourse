package atc

type Check struct {
	ID         int    `json:"id"`
	Status     string `json:"status"`
	CreateTime int64  `json:"create_time,omitempty"`
	StartTime  int64  `json:"start_time,omitempty"`
	EndTime    int64  `json:"end_time,omitempty"`
	CheckError string `json:"check_error,omitempty"`
}
