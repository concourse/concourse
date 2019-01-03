package concourse

import (
	"net/http"

	"github.com/concourse/concourse/go-concourse/concourse/internal"
)

func (client *client) UserInfo() (map[string]interface{}, error) {
	var connection = client.connection

	req, err := http.NewRequest("GET", connection.URL()+"/sky/userinfo", nil)
	if err != nil {
		return nil, err
	}

	var userInfo map[string]interface{}
	err = connection.SendHTTPRequest(req, false, &internal.Response{
		Result: &userInfo,
	})

	return userInfo, err
}
