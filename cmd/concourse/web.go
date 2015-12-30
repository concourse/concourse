package main

import (
	"net"
	"os"
	"time"

	"github.com/concourse/atc/atccmd"
	"github.com/concourse/tsa/tsacmd"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

type WebCommand struct {
	atccmd.ATCCommand

	TSA struct {
		BindIP   tsacmd.IPFlag `long:"bind-ip"   default:"0.0.0.0" description:"IP address on which to listen for SSH."`
		BindPort uint16        `long:"bind-port" default:"2222"    description:"Port on which to listen for SSH."`

		HostKeyPath        tsacmd.FileFlag `long:"host-key"               required:"true" description:"Key to use for the TSA's ssh server."`
		AuthorizedKeysPath tsacmd.FileFlag `long:"authorized-keys" required:"true" description:"Path to a file containing public keys to authorize for SSH access."`

		HeartbeatInterval time.Duration `long:"heartbeat-interval" default:"30s" description:"interval on which to heartbeat workers to the ATC"`
	} `group:"TSA Configuration" namespace:"tsa"`
}

func (cmd *WebCommand) Execute(args []string) error {
	tsa := &tsacmd.TSACommand{
		BindIP:   cmd.TSA.BindIP,
		BindPort: cmd.TSA.BindPort,

		HostKeyPath:        cmd.TSA.HostKeyPath,
		AuthorizedKeysPath: cmd.TSA.AuthorizedKeysPath,

		HeartbeatInterval: cmd.TSA.HeartbeatInterval,
	}

	cmd.populateTSAFlagsFromATCFlags(tsa)

	atcRunner, err := cmd.ATCCommand.Runner(args)
	if err != nil {
		return err
	}

	tsaRunner, err := tsa.Runner(args)
	if err != nil {
		return err
	}

	runner := sigmon.New(grouper.NewParallel(os.Interrupt, grouper.Members{
		{"atc", atcRunner},
		{"tsa", tsaRunner},
	}))

	return <-ifrit.Invoke(runner).Wait()
}

func (cmd *WebCommand) populateTSAFlagsFromATCFlags(tsa *tsacmd.TSACommand) error {
	// TODO: flag types package plz
	err := tsa.ATCURL.UnmarshalFlag(cmd.ATCCommand.PeerURL.String())
	if err != nil {
		return err
	}

	tsa.SessionSigningKeyPath = tsacmd.FileFlag(cmd.ATCCommand.SessionSigningKey)

	host, _, err := net.SplitHostPort(cmd.ATCCommand.PeerURL.URL().Host)
	if err != nil {
		return err
	}

	tsa.PeerIP = host

	tsa.Metrics.YellerAPIKey = cmd.ATCCommand.Metrics.YellerAPIKey
	tsa.Metrics.YellerEnvironment = cmd.ATCCommand.Metrics.YellerEnvironment

	return nil
}
