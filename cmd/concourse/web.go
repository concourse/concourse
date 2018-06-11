package main

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/concourse/atc/atccmd"
	"github.com/concourse/flag"
	"github.com/concourse/tsa/tsacmd"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"

	"github.com/concourse/bin/bindata"
)

type WebCommand struct {
	*atccmd.ATCCommand

	*tsacmd.TSACommand `group:"TSA Configuration" namespace:"tsa"`
}

const cliArtifactsBindata = "cli-artifacts"

func (WebCommand) lessenRequirements(command *flags.Command) {
	// defaults to address from external URL
	command.FindOptionByLongName("tsa-peer-ip").Required = false

	// defaults to atc external URL
	command.FindOptionByLongName("tsa-atc-url").Required = false

	// defaults to atc session signing key
	command.FindOptionByLongName("tsa-session-signing-key").Required = false
}

func (cmd *WebCommand) Execute(args []string) error {
	runner, err := cmd.Runner(args)
	if err != nil {
		return err
	}

	return <-ifrit.Invoke(sigmon.New(runner)).Wait()
}

func (cmd *WebCommand) Runner(args []string) (ifrit.Runner, error) {
	err := bindata.RestoreAssets(os.TempDir(), cliArtifactsBindata)
	if err == nil {
		cmd.ATCCommand.CLIArtifactsDir = flag.Dir(filepath.Join(os.TempDir(), cliArtifactsBindata))
	}

	cmd.populateTSAFlagsFromATCFlags()

	atcRunner, shouldSkipTSA, err := cmd.ATCCommand.Runner(args)
	if err != nil {
		return nil, err
	}

	if shouldSkipTSA {
		return atcRunner, nil
	}

	tsaRunner, err := cmd.TSACommand.Runner(args)
	if err != nil {
		return nil, err
	}

	return grouper.NewParallel(os.Interrupt, grouper.Members{
		{Name: "atc", Runner: atcRunner},
		{Name: "tsa", Runner: tsaRunner},
	}), nil
}

func (cmd *WebCommand) populateTSAFlagsFromATCFlags() error {
	cmd.TSACommand.SessionSigningKey = cmd.ATCCommand.Auth.AuthFlags.SigningKey

	if cmd.ATCCommand.Auth.AuthFlags.SigningKey.PrivateKey == nil &&
		cmd.TSACommand.SessionSigningKey.PrivateKey == nil {
		signingKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return fmt.Errorf("failed to generate session signing key: %s", err)
		}

		cmd.ATCCommand.Auth.AuthFlags.SigningKey = &flag.PrivateKey{PrivateKey: signingKey}
		cmd.TSACommand.SessionSigningKey = &flag.PrivateKey{PrivateKey: signingKey}
	}

	if len(cmd.TSACommand.ATCURLs) == 0 {
		cmd.TSACommand.ATCURLs = []flag.URL{cmd.ATCCommand.PeerURL}
	}

	host, _, err := net.SplitHostPort(cmd.ATCCommand.PeerURL.URL.Host)
	if err != nil {
		return err
	}

	cmd.TSACommand.PeerIP = host

	cmd.TSACommand.Metrics.YellerAPIKey = cmd.ATCCommand.Metrics.YellerAPIKey
	cmd.TSACommand.Metrics.YellerEnvironment = cmd.ATCCommand.Metrics.YellerEnvironment

	return nil
}
