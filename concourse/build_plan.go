package concourse

import (
	"strconv"

	"github.com/concourse/atc"
	"github.com/tedsuo/rata"
)

func (client *client) BuildPlan(buildID int) (atc.PublicBuildPlan, bool, error) {
	params := rata.Params{"build_id": strconv.Itoa(buildID)}
	var buildPlan atc.PublicBuildPlan
	err := client.connection.Send(Request{
		RequestName: atc.GetBuildPlan,
		Params:      params,
	}, &Response{
		Result: &buildPlan,
	})

	switch err.(type) {
	case nil:
		return buildPlan, true, nil
	case ResourceNotFoundError:
		return buildPlan, false, nil
	default:
		return buildPlan, false, err
	}
}
