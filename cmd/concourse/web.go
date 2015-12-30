package main

import (
	"net"
	"sync"
	"time"

	"github.com/concourse/atc/atccmd"
	"github.com/concourse/tsa/tsacmd"
	"github.com/hashicorp/go-multierror"
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

	errs := make(chan error, 2)

	wg := new(sync.WaitGroup)

	wg.Add(1)
	go func() {
		defer wg.Done()
		errs <- cmd.ATCCommand.Execute(args)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		errs <- tsa.Execute(nil)
	}()

	wg.Wait()

	var allErrors error

	for i := 0; i < 2; i++ {
		err := <-errs
		if err != nil {
			allErrors = multierror.Append(allErrors, err)
		}
	}

	return allErrors
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
