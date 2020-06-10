package concourse

import (
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
)

func (client *client) UserInfo() (atc.UserInfo, error) {
	var connection = client.connection

	req, err := http.NewRequest("GET", connection.URL()+"/api/v1/user", nil)
	if err != nil {
		return atc.UserInfo{}, err
	}

	var userInfo atc.UserInfo
	err = connection.SendHTTPRequest(req, false, &internal.Response{
		Result: &userInfo,
	})

	return userInfo, err
}
