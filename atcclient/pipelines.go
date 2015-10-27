package atcclient

import "github.com/concourse/atc"

func (client *client) ListPipelines() ([]atc.Pipeline, error) {
	var pipelines []atc.Pipeline
	err := client.connection.Send(Request{
		RequestName: atc.ListPipelines,
	}, &Response{
		Result: &pipelines,
	})

	return pipelines, err
}

func (client *client) DeletePipeline(pipelineName string) (bool, error) {
	params := map[string]string{"pipeline_name": pipelineName}
	err := client.connection.Send(Request{
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

func (client *client) PausePipeline(pipelineName string) (bool, error) {
	params := map[string]string{"pipeline_name": pipelineName}
	err := client.connection.Send(Request{
		RequestName: atc.PausePipeline,
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

func (handler AtcHandler) UnpausePipeline(pipelineName string) (bool, error) {
	params := map[string]string{"pipeline_name": pipelineName}
	err := handler.client.Send(Request{
		RequestName: atc.UnpausePipeline,
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
