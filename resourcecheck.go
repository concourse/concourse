package atc

type CheckRequestBody struct {
	From Version `json:"from"`
}

type CheckResponseBody struct {
	ExitStatus int    `json:"exit_status"`
	Stderr     string `json:"stderr"`
}
