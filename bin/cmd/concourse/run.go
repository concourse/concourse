package main

import (
	"os"
	"os/exec"

	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/sigmon"
)

func run(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	runner := sigmon.New(cmdRunner{cmd})

	process := ifrit.Invoke(runner)
	return <-process.Wait()
}
