package ssh

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/lager"
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
	Addrs               []string
	PrivateKeyFile      string
	UserKnownHostsFile  string
	ConnectTimeout      int
	ServerAliveInterval int
	ServerAliveCountMax int
	ConfigFile          string
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
		configFile, err := os.Open(r.options.ConfigFile)
		if err != nil {
			return err
		}

		cmd.Stdin = bufio.NewReader(configFile)
		cmd.Stdout = &LogWriter{logger: logger}

		err = cmd.Run()
		if err == nil {
			return nil
		}

		logger.Error("failed-to-run-ssh-command", err, lager.Data{"tsa-addr": addr})
	}

	return ErrFailedToReachAnyTSA
}
