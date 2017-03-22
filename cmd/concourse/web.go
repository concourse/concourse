package main

import (
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/concourse/atc/atccmd"
	"github.com/concourse/tsa/tsacmd"
	"github.com/concourse/tsa/tsaflags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"

	"github.com/concourse/bin/bindata"
)

type WebCommand struct {
	atccmd.ATCCommand

	TSA struct {
		BindIP   tsaflags.IPFlag `long:"bind-ip"   default:"0.0.0.0" description:"IP address on which to listen for SSH."`
		BindPort uint16        `long:"bind-port" default:"2222"    description:"Port on which to listen for SSH."`

		HostKeyPath            tsaflags.FileFlag        `long:"host-key"             required:"true" description:"Key to use for the TSA's ssh server."`
		AuthorizedKeysPath     tsaflags.FileFlag        `long:"authorized-keys"      required:"true" description:"Path to a file containing public keys to authorize for SSH access."`
		TeamAuthorizedKeysPath []tsaflags.InputPairFlag `long:"team-authorized-keys" value-name:"NAME=PATH" description:"Path to file containing keys to authorize, in SSH authorized_keys format (one public key per line)."`

		HeartbeatInterval time.Duration `long:"heartbeat-interval" default:"30s" description:"interval on which to heartbeat workers to the ATC"`
	} `group:"TSA Configuration" namespace:"tsa"`
}

const cliArtifactsBindata = "cli-artifacts"

func (cmd *WebCommand) Execute(args []string) error {
	err := bindata.RestoreAssets(os.TempDir(), cliArtifactsBindata)
	if err != nil {
		return err
	}

	cmd.ATCCommand.CLIArtifactsDir = atccmd.DirFlag(filepath.Join(os.TempDir(), cliArtifactsBindata))

	tsa := &tsacmd.TSACommand{
		BindIP:   cmd.TSA.BindIP,
		BindPort: cmd.TSA.BindPort,

		HostKeyPath:            cmd.TSA.HostKeyPath,
		AuthorizedKeysPath:     cmd.TSA.AuthorizedKeysPath,
		TeamAuthorizedKeysPath: cmd.TSA.TeamAuthorizedKeysPath,

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

	var f tsaflags.URLFlag
	err := f.UnmarshalFlag(cmd.ATCCommand.PeerURL.String())
	if err != nil {
		return err
	}

	tsa.ATCURLs = append(tsa.ATCURLs, f)

	tsa.SessionSigningKeyPath = tsaflags.FileFlag(cmd.ATCCommand.SessionSigningKey)

	host, _, err := net.SplitHostPort(cmd.ATCCommand.PeerURL.URL().Host)
	if err != nil {
		return err
	}

	tsa.PeerIP = host

	tsa.Metrics.YellerAPIKey = cmd.ATCCommand.Metrics.YellerAPIKey
	tsa.Metrics.YellerEnvironment = cmd.ATCCommand.Metrics.YellerEnvironment

	return nil
}
