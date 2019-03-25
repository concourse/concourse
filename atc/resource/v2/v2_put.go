package v2

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc"
)

type DecodeResponseError struct {
	Err error
}

func (e DecodeResponseError) Error() string {
	return fmt.Sprintf("failed to decode response: %s", e.Err.Error())
}

//go:generate counterfeiter . PutEventHandler

type PutEventHandler interface {
	CreatedResponse(atc.Space, atc.Version, atc.Metadata, []atc.SpaceVersion) ([]atc.SpaceVersion, error)
}

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
	config := constructConfig(source, params)
	input := PutRequest{config, "../" + responseFilename}
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

	process, err = r.container.Attach(ResourceProcessID, processIO)
	if err != nil {
		process, err = r.container.Run(garden.ProcessSpec{
			ID:   ResourceProcessID,
			Path: r.info.Artifacts.Put,
			Dir:  "put",
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
				Dir:        "put",
				ExitStatus: processStatus,

				Stderr: stderr.String(),
			}
		}

		out, err := r.container.StreamOut(garden.StreamOutSpec{Path: responseFilename})
		if err != nil {
			return nil, err
		}

		defer out.Close()

		tarReader := tar.NewReader(out)

		_, err = tarReader.Next()
		if err != nil {
			return nil, err
		}

		decoder := json.NewDecoder(tarReader)

		spaceVersions := []atc.SpaceVersion{}
		for {
			var event Event
			err := decoder.Decode(&event)
			if err != nil {
				if err == io.EOF {
					break
				}

				return nil, DecodeResponseError{Err: err}
			}

			if event.Action == "created" {
				spaceVersions, err = eventHandler.CreatedResponse(event.Space, event.Version, event.Metadata, spaceVersions)
				if err != nil {
					return nil, err
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
