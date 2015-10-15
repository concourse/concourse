package atcclient

import (
	"fmt"
	"net/http"

	"github.com/concourse/atc"
)

func (handler AtcHandler) DeletePipeline(pipelineName string) error {
	params := map[string]string{"pipeline_name": pipelineName}
	err := handler.client.MakeRequest(nil, atc.DeletePipeline, params, nil)

	if ure, ok := err.(UnexpectedResponseError); ok {
		if ure.StatusCode == http.StatusNotFound {
			return fmt.Errorf("`%s` does not exist", pipelineName)
		}
	}

	return err
}
