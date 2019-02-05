package tsacmd

import (
	"fmt"
	"net"
	"os"

	"code.cloudfoundry.org/lager"
)

type serverRunner struct {
	logger lager.Logger

	server *server

	listenAddr string
}

func (runner serverRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	listener, err := net.Listen("tcp", runner.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %s", runner.listenAddr, err)
	}

	runner.logger.Info("listening")

	close(ready)

	exited := make(chan error)

	go func() {
		exited <- runner.server.Serve(listener)
	}()

	for {
		select {
		case err := <-exited:
			return err
		case <-signals:
			listener.Close()
		}
	}
}
