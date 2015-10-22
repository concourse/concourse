package atcclient

import "github.com/concourse/atc"

func (handler AtcHandler) ListPipelines() ([]atc.Pipeline, error) {
	var pipelines []atc.Pipeline
	err := handler.client.Send(Request{
		RequestName: atc.ListPipelines,
	}, &Response{
		Result: &pipelines,
	})

	return pipelines, err
}

func (handler AtcHandler) DeletePipeline(pipelineName string) (bool, error) {
	params := map[string]string{"pipeline_name": pipelineName}
	err := handler.client.Send(Request{
		RequestName: atc.DeletePipeline,
		Params:      params,
	}, nil)

	switch err.(type) {
	case nil:
		return true, nil
	case ResourceNotFoundError:
		return false, nil
	default:
		return false, err
	}

}
