package concourse

import "github.com/concourse/atc"

func (client *client) Pipeline(pipelineName string) (atc.Pipeline, bool, error) {
	params := map[string]string{"pipeline_name": pipelineName}

	var pipeline atc.Pipeline
	err := client.connection.Send(Request{
		RequestName: atc.GetPipeline,
		Params:      params,
	}, &Response{
		Result: &pipeline,
	})

	switch err.(type) {
	case nil:
		return pipeline, true, nil
	case ResourceNotFoundError:
		return atc.Pipeline{}, false, nil
	default:
		return atc.Pipeline{}, false, err
	}
}

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

func (client *client) UnpausePipeline(pipelineName string) (bool, error) {
	params := map[string]string{"pipeline_name": pipelineName}
	err := client.connection.Send(Request{
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
