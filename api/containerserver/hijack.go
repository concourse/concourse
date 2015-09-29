package containerserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc"
	"github.com/pivotal-golang/lager"
)

func (s *Server) HijackContainer(w http.ResponseWriter, r *http.Request) {
	hijackRequest, err := s.parseHijackRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.hijack(w, hijackRequest)
}

type hijackRequest struct {
	ContainerHandle string
	Process         atc.HijackProcessSpec
}

func (s *Server) parseHijackRequest(r *http.Request) (hijackRequest, error) {
	handle := r.FormValue(":id")

	hLog := s.logger.Session("hijack", lager.Data{
		"handle": handle,
	})

	var processSpec atc.HijackProcessSpec
	err := json.NewDecoder(r.Body).Decode(&processSpec)
	if err != nil {
		hLog.Error("malformed-process-spec", err)
		return hijackRequest{}, fmt.Errorf("malformed process spec: %s", err)
	}

	return hijackRequest{
		ContainerHandle: handle,
		Process:         processSpec,
	}, nil
}

func (s *Server) hijack(w http.ResponseWriter, request hijackRequest) {
	hLog := s.logger.Session("hijack", lager.Data{
		"handle":  request.ContainerHandle,
		"process": request.Process,
	})

	container, err := s.workerClient.LookupContainer(hLog, request.ContainerHandle)
	if err != nil {
		hLog.Error("failed-to-get-container", err)
		if _, ok := err.(garden.ContainerNotFoundError); ok {
			http.Error(w, fmt.Sprintf("failed to get container: %s", err), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("failed to get container: %s", err), http.StatusInternalServerError)
		}
		return
	}

	defer container.Release()

	w.WriteHeader(http.StatusOK)

	conn, br, err := w.(http.Hijacker).Hijack()
	if err != nil {
		hLog.Error("failed-to-hijack", err)
		return
	}

	defer conn.Close()

	stdinR, stdinW := io.Pipe()

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(br)

	inputs := make(chan atc.HijackInput)
	outputs := make(chan atc.HijackOutput)
	exited := make(chan int, 1)
	errs := make(chan error, 1)

	cleanup := make(chan struct{})
	defer close(cleanup)

	outW := &stdoutWriter{
		outputs: outputs,
		done:    cleanup,
	}

	errW := &stderrWriter{
		outputs: outputs,
		done:    cleanup,
	}

	var tty *garden.TTYSpec

	if request.Process.TTY != nil {
		tty = &garden.TTYSpec{
			WindowSize: &garden.WindowSize{
				Columns: request.Process.TTY.WindowSize.Columns,
				Rows:    request.Process.TTY.WindowSize.Rows,
			},
		}
	}

	process, err := container.Run(garden.ProcessSpec{
		Path: request.Process.Path,
		Args: request.Process.Args,
		Env:  request.Process.Env,
		Dir:  request.Process.Dir,

		User: request.Process.User,

		TTY: tty,
	}, garden.ProcessIO{
		Stdin:  stdinR,
		Stdout: outW,
		Stderr: errW,
	})
	if err != nil {
		hLog.Error("failed-to-hijack", err)
		return
	}

	hLog.Info("hijacked")

	go func() {
		for {
			var input atc.HijackInput
			err := dec.Decode(&input)
			if err != nil {
				break
			}

			select {
			case inputs <- input:
			case <-cleanup:
				return
			}
		}
	}()

	go func() {
		status, err := process.Wait()
		if err != nil {
			errs <- err
		} else {
			exited <- status
		}
	}()

	for {
		select {
		case input := <-inputs:
			if input.TTYSpec != nil {
				err := process.SetTTY(garden.TTYSpec{
					WindowSize: &garden.WindowSize{
						Columns: input.TTYSpec.WindowSize.Columns,
						Rows:    input.TTYSpec.WindowSize.Rows,
					},
				})
				if err != nil {
					enc.Encode(atc.HijackOutput{
						Error: err.Error(),
					})
				}
			} else {
				stdinW.Write(input.Stdin)
			}

		case output := <-outputs:
			err := enc.Encode(output)
			if err != nil {
				return
			}

		case status := <-exited:
			enc.Encode(atc.HijackOutput{
				ExitStatus: &status,
			})

			return

		case err := <-errs:
			enc.Encode(atc.HijackOutput{
				Error: err.Error(),
			})

			return
		}
	}
}

type stdoutWriter struct {
	outputs chan<- atc.HijackOutput
	done    chan struct{}
}

func (writer *stdoutWriter) Write(b []byte) (int, error) {
	chunk := make([]byte, len(b))
	copy(chunk, b)

	output := atc.HijackOutput{
		Stdout: chunk,
	}

	select {
	case writer.outputs <- output:
	case <-writer.done:
	}

	return len(b), nil
}

func (writer *stdoutWriter) Close() error {
	close(writer.done)
	return nil
}

type stderrWriter struct {
	outputs chan<- atc.HijackOutput
	done    chan struct{}
}

func (writer *stderrWriter) Write(b []byte) (int, error) {
	chunk := make([]byte, len(b))
	copy(chunk, b)

	output := atc.HijackOutput{
		Stderr: chunk,
	}

	select {
	case writer.outputs <- output:
	case <-writer.done:
	}

	return len(b), nil
}

func (writer *stderrWriter) Close() error {
	close(writer.done)
	return nil
}
