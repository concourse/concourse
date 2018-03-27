package ssh

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
)

var ErrFailedToReachAnyTSA = errors.New("failed-to-reach-any-tsa-assuming-cluster-is-being-destroyed")

type LogWriter struct {
	logger lager.Logger
}

func (lw *LogWriter) Write(p []byte) (n int, err error) {
	lw.logger.Debug(string(p))
	return len(p), nil
}

//go:generate counterfeiter . Runner

type Runner interface {
	RetireWorker(logger lager.Logger) error
	LandWorker(logger lager.Logger) error
	DeleteWorker(logger lager.Logger) error
}

type runner struct {
	options Options
}

func NewRunner(options Options) Runner {
	return &runner{
		options: options,
	}
}

type Options struct {
	Name                string
	CertsPath           string
	Addrs               []string
	PrivateKeyFile      string
	UserKnownHostsFile  string
	ConnectTimeout      int
	ServerAliveInterval int
	ServerAliveCountMax int
}

func (r *runner) RetireWorker(logger lager.Logger) error {
	return r.run(logger, "retire-worker")
}

func (r *runner) LandWorker(logger lager.Logger) error {
	return r.run(logger, "land-worker")
}

func (r *runner) DeleteWorker(logger lager.Logger) error {
	return r.run(logger, "delete-worker")
}

func (r *runner) run(logger lager.Logger, commandName string) error {
	for _, addr := range r.options.Addrs {
		addrParts := strings.Split(addr, ":")
		addr := addrParts[0]
		port := addrParts[1]

		workerJson, err := json.Marshal(atc.Worker{
			Name:      r.options.Name,
			CertsPath: &r.options.CertsPath,
		})
		if err != nil {
			return err
		}

		cmd := exec.Command("ssh",
			"-p", port,
			addr,
			"-i", r.options.PrivateKeyFile,
			"-o", fmt.Sprintf("UserKnownHostsFile=%s", r.options.UserKnownHostsFile),
			"-o", fmt.Sprintf("ConnectTimeout=%d", r.options.ConnectTimeout),
			"-o", fmt.Sprintf("ServerAliveInterval=%d", r.options.ServerAliveInterval),
			"-o", fmt.Sprintf("ServerAliveCountMax=%d", r.options.ServerAliveCountMax),
			commandName,
		)

		cmd.Stdin = bytes.NewBuffer(workerJson)
		cmd.Stdout = &LogWriter{logger: logger}

		err = cmd.Run()
		if err == nil {
			return nil
		}

		logger.Error("failed-to-run-ssh-command", err, lager.Data{"tsa-addr": addr})
	}

	return ErrFailedToReachAnyTSA
}
