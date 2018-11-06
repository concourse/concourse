package v2

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc"
)

//go:generate counterfeiter . CheckEventHandler

type CheckEventHandler interface {
	DefaultSpace(atc.Space) error
	Discovered(atc.Space, atc.Version, atc.Metadata) error
	LatestVersions() error
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
	tmpfile, err := ioutil.TempFile("", "response")
	if err != nil {
		return err
	}

	defer os.Remove(tmpfile.Name())

	path := r.info.Artifacts.Check
	input := CheckRequest{src, from, tmpfile.Name()}

	request, err := json.Marshal(input)
	if err != nil {
		return err
	}

	stderr := new(bytes.Buffer)

	processIO := garden.ProcessIO{
		Stdin:  bytes.NewBuffer(request),
		Stderr: stderr,
	}

	process, err := r.container.Run(garden.ProcessSpec{
		Path: path,
	}, processIO)
	if err != nil {
		return err
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
				Path:       path,
				ExitStatus: processStatus,

				Stderr: stderr.String(),
			}
		}

		fileReader, err := os.Open(tmpfile.Name())
		if err != nil {
			return err
		}

		decoder := json.NewDecoder(fileReader)

		for {
			var event Event
			err := decoder.Decode(&event)
			if err != nil {
				if err == io.EOF {
					break
				}

				return err
			}

			err = r.handleCheckEvent(event, checkHandler)
			if err != nil {
				return err
			}
		}

		err = checkHandler.LatestVersions()
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
		err := checkHandler.DefaultSpace(event.Space)
		return err

	case "discovered":
		err := checkHandler.Discovered(event.Space, event.Version, event.Metadata)
		return err
	}

	return ActionNotFoundError{event.Action}
}
