package concourse

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/concourse/concourse/v8/atc"
	"github.com/concourse/concourse/v8/go-concourse/concourse/internal"
)

func (client *client) GetHealth() (atc.Health, error) {
	resp, err := client.httpAgent.Send(internal.Request{
		RequestName: atc.GetHealth,
	})
	if err != nil {
		return atc.Health{}, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusServiceUnavailable:
		var health atc.Health
		if err = json.NewDecoder(resp.Body).Decode(&health); err != nil {
			return atc.Health{}, err
		}
		return health, nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return atc.Health{}, internal.UnexpectedResponseError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}
	}
}
