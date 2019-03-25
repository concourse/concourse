package v2

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc"
)

//go:generate counterfeiter . CheckEventHandler

type CheckEventHandler interface {
	SaveDefault(atc.Space) error
	Save(atc.Space, atc.Version, atc.Metadata) error
	Finish() error
}

type ResourceVersion struct {
	Space    string       `json:"space"`
	Version  atc.Version  `json:"version"`
	Metadata atc.Metadata `json:"metadata"`
}

type CheckRequest struct {
	Config       map[string]interface{}    `json:"config"`
	From         map[atc.Space]atc.Version `json:"from"`
	ResponsePath string                    `json:"response_path"`
}

func (r *resource) Check(
	ctx context.Context,
	checkHandler CheckEventHandler,
	src atc.Source,
	from map[atc.Space]atc.Version,
) error {
	input := CheckRequest{src, from, "../" + responseFilename}

	request, err := json.Marshal(input)
	if err != nil {
		return err
	}

	stderr := new(bytes.Buffer)

	processIO := garden.ProcessIO{
		Stdin:  bytes.NewBuffer(request),
		Stdout: stderr,
		Stderr: stderr,
	}

	var process garden.Process

	process, err = r.container.Attach(ResourceProcessID, processIO)
	if err != nil {
		process, err = r.container.Run(garden.ProcessSpec{
			ID:   ResourceProcessID,
			Path: r.info.Artifacts.Check,
			Dir:  "check",
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
				Path:       r.info.Artifacts.Check,
				Dir:        "check",
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

		for {
			var event Event
			err := decoder.Decode(&event)
			if err != nil {
				if err == io.EOF {
					break
				}

				return DecodeResponseError{Err: err}
			}

			err = r.handleCheckEvent(event, checkHandler)
			if err != nil {
				return err
			}
		}

		err = checkHandler.Finish()
		if err != nil {
			return err
		}

		return nil

	case <-ctx.Done():
		r.container.Stop(false)
		<-processExited
		return ctx.Err()
	}
}

func (r *resource) handleCheckEvent(event Event, checkHandler CheckEventHandler) error {
	switch action := event.Action; action {
	case "default_space":
		err := checkHandler.SaveDefault(event.Space)
		return err

	case "discovered":
		err := checkHandler.Save(event.Space, event.Version, event.Metadata)
		return err
	}

	return ActionNotFoundError{event.Action}
}
