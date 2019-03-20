package concourse

import (
	"io"
	"net/http"
	"strconv"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) CreateArtifact(src io.Reader) (atc.WorkerArtifact, error) {
	var artifact atc.WorkerArtifact

	params := rata.Params{
		"team_name": team.Name(),
	}

	err := team.connection.Send(internal.Request{
		Header:      http.Header{"Content-Type": {"application/octet-stream"}},
		RequestName: atc.CreateArtifact,
		Params:      params,
		Body:        src,
	}, &internal.Response{
		Result: &artifact,
	})

	return artifact, err
}

func (team *team) GetArtifact(artifactID int) (io.ReadCloser, error) {
	params := rata.Params{
		"team_name":   team.Name(),
		"artifact_id": strconv.Itoa(artifactID),
	}

	response := internal.Response{}
	err := team.connection.Send(internal.Request{
		RequestName:        atc.GetArtifact,
		Params:             params,
		ReturnResponseBody: true,
	}, &response)

	if err != nil {
		return nil, err
	}

	return response.Result.(io.ReadCloser), nil
}
