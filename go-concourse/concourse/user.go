package concourse

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
)

func (client *client) UserInfo() (atc.UserInfo, error) {
	resp, err := client.httpAgent.Send(internal.Request{
		RequestName: atc.GetUser,
	})

	if err != nil {
		return atc.UserInfo{}, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var userInfo atc.UserInfo
		err = json.NewDecoder(resp.Body).Decode(&userInfo)
		if err != nil {
			return atc.UserInfo{}, err
		}
		return userInfo, nil
	case http.StatusUnauthorized:
		return atc.UserInfo{}, ErrUnauthorized
	default:
		body, _ := ioutil.ReadAll(resp.Body)
		return atc.UserInfo{}, internal.UnexpectedResponseError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}
	}
}
