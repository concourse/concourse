package concourse

import (
	"io"
	"net/http"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (client *client) BuildPlan(buildID int) (atc.PublicBuildPlan, bool, error) {
	params := rata.Params{
		"build_id": strconv.Itoa(buildID),
	}

	var buildPlan atc.PublicBuildPlan
	err := client.connection.Send(internal.Request{
		RequestName: atc.GetBuildPlan,
		Params:      params,
	}, &internal.Response{
		Result: &buildPlan,
	})

	switch err.(type) {
	case nil:
		return buildPlan, true, nil
	case internal.ResourceNotFoundError:
		return buildPlan, false, nil
	default:
		return buildPlan, false, err
	}
}

func (client *client) SendInputToBuildPlan(buildID int, planID atc.PlanID, src io.Reader) (bool, error) {
	params := rata.Params{
		"build_id": strconv.Itoa(buildID),
		"plan_id":  string(planID),
	}

	response := internal.Response{}
	err := client.connection.Send(internal.Request{
		Header:      http.Header{"Content-Type": {"application/octet-stream"}},
		RequestName: atc.SendInputToBuildPlan,
		Params:      params,
		Body:        src,
	}, &response)

	switch err.(type) {
	case nil:
		return true, nil
	case internal.ResourceNotFoundError:
		return false, nil
	default:
		return false, err
	}
}

func (client *client) ReadOutputFromBuildPlan(buildID int, planID atc.PlanID) (io.ReadCloser, bool, error) {
	params := rata.Params{
		"build_id": strconv.Itoa(buildID),
		"plan_id":  string(planID),
	}

	response := internal.Response{}
	err := client.connection.Send(internal.Request{
		RequestName:        atc.ReadOutputFromBuildPlan,
		Params:             params,
		ReturnResponseBody: true,
	}, &response)

	switch err.(type) {
	case nil:
		return response.Result.(io.ReadCloser), true, nil
	case internal.ResourceNotFoundError:
		return nil, false, nil
	default:
		return nil, false, err
	}
}
