package main

import (
	"fmt"
	"os"

	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

type ConcourseCommand struct {
	WorkDir string `long:"work-dir" description:"Directory in which to place runtime data." required:"true"`
	PeerIP  string `long:"peer-ip" default:"127.0.0.1" description:"IP used to reach this node from other Concourse nodes"`
	User    string `long:"user" description:"User to run unprivileged processes as." required:"true"`

	ATCArgs []string
}

func (cmd *ConcourseCommand) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	atcCmd, err := cmd.atc()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to prepare atc: %s\n", err)
		os.Exit(1)
	}

	gardenCmd, err := cmd.garden()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to prepare garden: %s\n", err)
		os.Exit(1)
	}

	atcCmd.Stdout = os.Stdout
	atcCmd.Stderr = os.Stderr

	gardenCmd.Stdout = os.Stdout
	gardenCmd.Stderr = os.Stderr

	// TODO: this treats any signal as fatal; will need platform-specific signals
	// listed otherwise
	runner := sigmon.New(grouper.NewParallel(os.Interrupt, grouper.Members{
		{"atc", cmdRunner{atcCmd}},
		{"garden", cmdRunner{gardenCmd}},
	}))

	return runner.Run(signals, ready)
}
