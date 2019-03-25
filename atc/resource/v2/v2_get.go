package v2

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/worker"
)

//go:generate counterfeiter . GetEventHandler

type GetEventHandler interface {
	SaveMetadata(atc.Metadata) error
}

type GetRequest struct {
	Config       map[string]interface{} `json:"config"`
	Space        atc.Space              `json:"space"`
	Version      atc.Version            `json:"version,omitempty"`
	ResponsePath string                 `json:"response_path"`
}

func (r *resource) Get(
	ctx context.Context,
	eventHandler GetEventHandler,
	volume worker.Volume,
	ioConfig atc.IOConfig,
	source atc.Source,
	params atc.Params,
	space atc.Space,
	version atc.Version,
) error {
	config := constructConfig(source, params)
	input := GetRequest{config, space, version, "../" + responseFilename}
	request, err := json.Marshal(input)
	if err != nil {
		return err
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
			Path: r.info.Artifacts.Get,
			Dir:  "get",
		}, processIO)
		if err != nil {
			return err
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
			return processErr
		}

		if processStatus != 0 {
			return atc.ErrResourceScriptFailed{
				Path:       r.info.Artifacts.Get,
				Dir:        "get",
				ExitStatus: processStatus,

				Stderr: stderr.String(),
			}
		}

		out, err := r.container.StreamOut(garden.StreamOutSpec{Path: responseFilename})
		if err != nil {
			return err
		}

		defer out.Close()

		tarReader := tar.NewReader(out)

		_, err = tarReader.Next()
		if err != nil {
			return err
		}

		decoder := json.NewDecoder(tarReader)

		var event Event
		err = decoder.Decode(&event)
		if err != nil {
			return DecodeResponseError{Err: err}
		}

		if event.Action == "fetched" {
			err = eventHandler.SaveMetadata(event.Metadata)
			if err != nil {
				return err
			}
		} else {
			return ActionNotFoundError{event.Action}
		}

		return nil

	case <-ctx.Done():
		r.container.Stop(false)
		<-processExited
		return ctx.Err()
	}
}
