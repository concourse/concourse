package atc

type CheckRequestBody struct {
	From    Version `json:"from"`
	Shallow bool    `json:"shallow"`
}
