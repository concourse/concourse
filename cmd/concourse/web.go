package main

import (
	"os"

	concourseCmd "github.com/concourse/concourse/cmd"

	"github.com/concourse/concourse/atc/atccmd"
	"github.com/concourse/concourse/tsa/tsacmd"
	"github.com/concourse/flag"
	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

type WebCommand struct {
	PeerAddress string `long:"peer-address" default:"127.0.0.1" description:"Network address of this web node, reachable by other web nodes. Used for forwarded worker addresses."`

	*atccmd.RunCommand
	*tsacmd.TSACommand `group:"TSA Configuration" namespace:"tsa"`
}

func (WebCommand) LessenRequirements(command *flags.Command) {
	// defaults to atc external URL
	command.FindOptionByLongName("tsa-atc-url").Required = false
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
		cmd.RunCommand.CLIArtifactsDir = flag.Dir(concourseCmd.DiscoverAsset("fly-assets"))
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
			Runner: concourseCmd.NewLoggingRunner(logger.Session("atc-runner"), atcRunner),
		},
		{
			Name:   "tsa",
			Runner: concourseCmd.NewLoggingRunner(logger.Session("tsa-runner"), tsaRunner),
		},
	}), nil
}

func (cmd *WebCommand) populateTSAFlagsFromATCFlags() error {
	cmd.TSACommand.PeerAddress = cmd.PeerAddress

	if len(cmd.TSACommand.ATCURLs) == 0 {
		cmd.TSACommand.ATCURLs = []flag.URL{cmd.RunCommand.DefaultURL()}
	}

	cmd.TSACommand.ClusterName = cmd.RunCommand.Server.ClusterName
	cmd.TSACommand.LogClusterName = cmd.RunCommand.LogClusterName

	return nil
}
