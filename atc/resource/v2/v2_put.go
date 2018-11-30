package v2

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc"
)

//go:generate counterfeiter . PutEventHandler

type PutEventHandler interface {
	CreatedResponse(atc.Space, atc.Version, atc.Metadata, []atc.SpaceVersion) ([]atc.SpaceVersion, error)
}

const responsePath = "response"

type PutRequest struct {
	Config       map[string]interface{} `json:"config"`
	ResponsePath string                 `json:"response_path"`
}

func (r *resource) Put(
	ctx context.Context,
	eventHandler PutEventHandler,
	ioConfig atc.IOConfig,
	source atc.Source,
	params atc.Params,
) ([]atc.SpaceVersion, error) {
	var responseFile *os.File

	_, err := os.Stat(responsePath)
	if err == nil {
		responseFile, err = os.Open(responsePath)
	} else if os.IsNotExist(err) {
		responseFile, err = os.Create(responsePath)
	}

	defer responseFile.Close()

	if err != nil {
		return nil, err
	}

	config := constructConfig(source, params)
	input := PutRequest{config, responseFile.Name()}
	request, err := json.Marshal(input)
	if err != nil {
		return nil, err
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
			return nil, err
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
			return nil, processErr
		}

		if processStatus != 0 {
			return nil, atc.ErrResourceScriptFailed{
				Path:       r.info.Artifacts.Put,
				Args:       []string{atc.ResourcesDir("put")},
				ExitStatus: processStatus,

				Stderr: stderr.String(),
			}
		}

		fileReader, err := os.Open(responseFile.Name())
		if err != nil {
			return nil, err
		}

		decoder := json.NewDecoder(fileReader)

		spaceVersions := []atc.SpaceVersion{}
		for {
			var event Event
			err := decoder.Decode(&event)
			if err != nil {
				if err == io.EOF {
					break
				}

				return nil, err
			}

			if event.Action == "created" {
				spaceVersions, err = eventHandler.CreatedResponse(event.Space, event.Version, event.Metadata, spaceVersions)
				if err != nil {
					return nil, nil
				}
			} else {
				return nil, ActionNotFoundError{event.Action}
			}
		}

		return spaceVersions, nil

	case <-ctx.Done():
		r.container.Stop(false)
		<-processExited
		return nil, ctx.Err()
	}
}
