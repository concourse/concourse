package atc

type CheckRequestBody struct {
	From  Version `json:"from"`
	Space Space   `json:"space"`
}

type CheckResponseBody struct {
	ExitStatus int    `json:"exit_status"`
	Stderr     string `json:"stderr"`
}
