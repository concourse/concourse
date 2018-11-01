package v2

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc"
)

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

		if decoder.More() {
			var defaultSpace atc.DefaultSpaceResponse
			err := decoder.Decode(&defaultSpace)
			if err != nil {
				return err
			}

			if defaultSpace.DefaultSpace != "" {
				err = r.resourceConfig.SaveDefaultSpace(defaultSpace.DefaultSpace)
				if err != nil {
					return err
				}
			}
		}

		spaces := make(map[atc.Space]bool)
		for decoder.More() {
			var version atc.SpaceVersion
			err := decoder.Decode(&version)
			if err != nil {
				return err
			}

			if _, ok := spaces[version.Space]; !ok {
				err = r.resourceConfig.SaveSpace(version.Space)
				if err != nil {
					return err
				}
			}

			spaces[version.Space] = true

			err = r.resourceConfig.SaveVersion(version)
			if err != nil {
				return err
			}
		}

		return nil

	case <-ctx.Done():
		r.container.Stop(false)
		<-processExited
		return ctx.Err()
	}
}
