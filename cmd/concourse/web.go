package main

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"os"

	"github.com/concourse/concourse/v5/atc/atccmd"
	"github.com/concourse/concourse/v5/tsa/tsacmd"
	"github.com/concourse/flag"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

type WebCommand struct {
	PeerAddress string `long:"peer-address" default:"127.0.0.1" description:"Network address of this web node, reachable by other web nodes. Used for forwarded worker addresses."`

	*atccmd.RunCommand
	*tsacmd.TSACommand `group:"TSA Configuration" namespace:"tsa"`
}

func (WebCommand) lessenRequirements(command *flags.Command) {
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
	if cmd.RunCommand.CLIArtifactsDir == "" {
		cmd.RunCommand.CLIArtifactsDir = flag.Dir(discoverAsset("fly-assets"))
	}

	cmd.populateTSAFlagsFromATCFlags()

	atcRunner, err := cmd.RunCommand.Runner(args)
	if err != nil {
		return nil, err
	}

	tsaRunner, err := cmd.TSACommand.Runner(args)
	if err != nil {
		return nil, err
	}

	logger, _ := cmd.RunCommand.Logger.Logger("web")
	return grouper.NewParallel(os.Interrupt, grouper.Members{
		{
			Name:   "atc",
			Runner: NewLoggingRunner(logger.Session("atc-runner"), atcRunner),
		},
		{
			Name:   "tsa",
			Runner: NewLoggingRunner(logger.Session("tsa-runner"), tsaRunner),
		},
	}), nil
}

func (cmd *WebCommand) populateTSAFlagsFromATCFlags() error {
	cmd.TSACommand.SessionSigningKey = cmd.RunCommand.Auth.AuthFlags.SigningKey
	cmd.TSACommand.PeerAddress = cmd.PeerAddress

	if (cmd.RunCommand.Auth.AuthFlags.SigningKey == nil || cmd.RunCommand.Auth.AuthFlags.SigningKey.PrivateKey == nil) &&
		(cmd.TSACommand.SessionSigningKey == nil || cmd.TSACommand.SessionSigningKey.PrivateKey == nil) {
		signingKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return fmt.Errorf("failed to generate session signing key: %s", err)
		}

		cmd.RunCommand.Auth.AuthFlags.SigningKey = &flag.PrivateKey{PrivateKey: signingKey}
		cmd.TSACommand.SessionSigningKey = &flag.PrivateKey{PrivateKey: signingKey}
	}

	if len(cmd.TSACommand.ATCURLs) == 0 {
		cmd.TSACommand.ATCURLs = []flag.URL{cmd.RunCommand.DefaultURL()}
	}

	return nil
}
