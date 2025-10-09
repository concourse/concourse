package concourse

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
)

func (client *client) GetWall() (atc.Wall, error) {
	var wall atc.Wall

	err := client.connection.Send(internal.Request{
		RequestName: atc.GetWall,
	}, &internal.Response{
		Result: &wall,
	})

	return wall, err
}

func (client *client) SetWall(wall atc.Wall) error {
	buffer := &bytes.Buffer{}
	err := json.NewEncoder(buffer).Encode(wall)
	if err != nil {
		return err
	}

	err = client.connection.Send(internal.Request{
		RequestName: atc.SetWall,
		Body:        buffer,
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	}, &internal.Response{})

	return err
}

func (client *client) ClearWall() error {
	err := client.connection.Send(internal.Request{
		RequestName: atc.ClearWall,
	}, &internal.Response{})

	return err
}
