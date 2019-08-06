package concourse

import (
	"net/http"
	"net/url"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
)

const inputDateLayout = "2006-01-02"

func (client *client) ListActiveUsersSince(since time.Time) ([]atc.User, error) {
	var users []atc.User

	queryParams := url.Values{}
	queryParams.Add("since", since.Format(inputDateLayout))

	err := client.connection.Send(internal.Request{
		RequestName: atc.ListActiveUsersSince,
		Query:       queryParams,
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	}, &internal.Response{
		Result: &users,
	})
	if err != nil {
		return nil, err
	}

	return users, nil
}
