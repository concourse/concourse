package concourse

import (
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (client *client) Check(checkID string) (atc.Check, bool, error) {

	params := rata.Params{
		"check_id": checkID,
	}

	var check atc.Check
	err := client.connection.Send(internal.Request{
		RequestName: atc.GetCheck,
		Params:      params,
	}, &internal.Response{
		Result: &check,
	})

	switch e := err.(type) {
	case nil:
		return check, true, nil
	case internal.ResourceNotFoundError:
		return check, false, nil
	case internal.UnexpectedResponseError:
		if e.StatusCode == http.StatusInternalServerError {
			return check, false, GenericError{e.Body}
		} else {
			return check, false, err
		}
	default:
		return check, false, err
	}
}
