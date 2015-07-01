package hijackserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
)

func (s *Server) Hijack(w http.ResponseWriter, r *http.Request) {
	workerIdentifier := worker.Identifier{
		Type: worker.ContainerType(r.URL.Query().Get("type")),
		Name: r.URL.Query().Get("name"),
	}

	var err error

	buildIDParam := r.URL.Query().Get("build-id")
	if len(buildIDParam) != 0 {
		workerIdentifier.BuildID, err = strconv.Atoi(buildIDParam)
		if err != nil {
			http.Error(w, fmt.Sprintf("malformed build ID: %s", err), http.StatusBadRequest)
			return
		}
	}

	hLog := s.logger.Session("hijack", lager.Data{
		"identifier": workerIdentifier,
	})

	var processSpec atc.HijackProcessSpec
	err = json.NewDecoder(r.Body).Decode(&processSpec)
	if err != nil {
		hLog.Error("malformed-process-spec", err)
		http.Error(w, fmt.Sprintf("malformed process spec: %s", err), http.StatusBadRequest)
		return
	}

	container, err := s.workerClient.LookupContainer(workerIdentifier)
	if err != nil {
		hLog.Error("failed-to-get-container", err)
		http.Error(w, fmt.Sprintf("failed to get container: %s", err), http.StatusNotFound)
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

	if processSpec.TTY != nil {
		tty = &garden.TTYSpec{
			WindowSize: &garden.WindowSize{
				Columns: processSpec.TTY.WindowSize.Columns,
				Rows:    processSpec.TTY.WindowSize.Rows,
			},
		}
	}

	process, err := container.Run(garden.ProcessSpec{
		Path: processSpec.Path,
		Args: processSpec.Args,
		Env:  processSpec.Env,
		Dir:  processSpec.Dir,

		Privileged: processSpec.Privileged,
		User:       processSpec.User,

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
