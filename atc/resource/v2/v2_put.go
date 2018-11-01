package v2

import (
	"bytes"
	"context"
	"encoding/json"
	"os"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc"
)

const responsePath = "response"

type PutRequest struct {
	Config       map[string]interface{} `json:"config"`
	ResponsePath string                 `json:"response_path"`
}

type SpaceResponse struct {
	Space atc.Space `json:"space"`
}

type VersionResponse struct {
	Type    string      `json:"type"`
	Version atc.Version `json:"version"`
}

func (r *resource) Put(
	ctx context.Context,
	ioConfig atc.IOConfig,
	source atc.Source,
	params atc.Params,
) (atc.PutResponse, error) {
	var responseFile *os.File

	_, err := os.Stat(responsePath)
	if err == nil {
		responseFile, err = os.Open(responsePath)
	} else if os.IsNotExist(err) {
		responseFile, err = os.Create(responsePath)
	}

	defer responseFile.Close()

	if err != nil {
		return atc.PutResponse{}, err
	}

	config := constructConfig(source, params)
	input := PutRequest{config, responseFile.Name()}
	request, err := json.Marshal(input)
	if err != nil {
		return atc.PutResponse{}, err
	}

	stderr := new(bytes.Buffer)

	processIO := garden.ProcessIO{
		Stdin: bytes.NewBuffer(request),
	}

	if ioConfig.Stderr != nil {
		processIO.Stderr = ioConfig.Stderr
	} else {
		processIO.Stderr = stderr
	}

	var process garden.Process

	process, err = r.container.Attach(TaskProcessID, processIO)
	if err != nil {
		process, err = r.container.Run(garden.ProcessSpec{
			ID:   TaskProcessID,
			Path: r.info.Artifacts.Put,
			Args: []string{atc.ResourcesDir("put")},
		}, processIO)
		if err != nil {
			return atc.PutResponse{}, err
		}
	}

	processExited := make(chan struct{})

	var processStatus int
	var processErr error

	go func() {
		processStatus, processErr = process.Wait()
		close(processExited)
	}()

	select {
	case <-processExited:
		if processErr != nil {
			return atc.PutResponse{}, processErr
		}

		if processStatus != 0 {
			return atc.PutResponse{}, atc.ErrResourceScriptFailed{
				Path:       r.info.Artifacts.Put,
				Args:       []string{atc.ResourcesDir("put")},
				ExitStatus: processStatus,

				Stderr: stderr.String(),
			}
		}

		fileReader, err := os.Open(responseFile.Name())
		if err != nil {
			return atc.PutResponse{}, err
		}

		decoder := json.NewDecoder(fileReader)

		var response atc.PutResponse
		if decoder.More() {
			var spaceResponse SpaceResponse
			err := decoder.Decode(&spaceResponse)
			if err != nil {
				return atc.PutResponse{}, err
			}

			response.Space = spaceResponse.Space
		}

		for decoder.More() {
			var versionResponse VersionResponse
			err := decoder.Decode(&versionResponse)
			if err != nil {
				return atc.PutResponse{}, err
			}

			response.CreatedVersions = append(response.CreatedVersions, versionResponse.Version)
		}

		return response, nil

	case <-ctx.Done():
		r.container.Stop(false)
		<-processExited
		return atc.PutResponse{}, ctx.Err()
	}
}
