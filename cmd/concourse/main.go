package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
)

// FLAGS NEEDED FOR CLUSTER:
// --peer-ip for --peer-url and local worker registration
//
// --session-signing-key so all ATCs trust each other's tokens
//
// TODON'T: tsa server
// TODO: tsa client
// TODO: baggageclaim
// TODO: fly cli downloads
func main() {
	cmd := &ConcourseCommand{}

	parser := flags.NewParser(cmd, flags.Default)
	parser.NamespaceDelimiter = "-"

	atcArgs, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}

	cmd.ATCArgs = atcArgs

	running := ifrit.Invoke(cmd)

	err = <-running.Wait()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type cmdRunner struct {
	cmd *exec.Cmd
}

func (runner cmdRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := runner.cmd.Start()
	if err != nil {
		return err
	}

	close(ready)

	waitErr := make(chan error, 1)

	go func() {
		waitErr <- runner.cmd.Wait()
	}()

	for {
		select {
		case sig := <-signals:
			runner.cmd.Process.Signal(sig)
		case err := <-waitErr:
			return err
		}
	}
}
