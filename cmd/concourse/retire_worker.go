package main

import (
	"context"
	"os"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/worker"
	"github.com/spf13/cobra"
)

var retireWorkerCmd RetireWorkerConfig

var RetireWorkerCommand = &cobra.Command{
	Use:   "retire-worker",
	Short: "Safely remove a worker from the cluster permanently",
	Long:  `TODO`,
	RunE:  ExecuteRetireWorker,
}

func init() {
	RetireWorkerCommand.Flags().StringVar(&retireWorkerCmd.WorkerName, "name", "", "The name of the worker you wish to retire.")
	RetireWorkerCommand.Flags().StringVar(&retireWorkerCmd.WorkerTeam, "team", "", "The team name of the worker you wish to retire.")
	RetireWorkerCommand.Flags().StringSliceVar(&retireWorkerCmd.TSA.Hosts, "tsa-host", []string{"127.0.0.1:2222"}, "TSA host to forward the worker through. Can be specified multiple times.")
	RetireWorkerCommand.Flags().Var(&retireWorkerCmd.TSA.PublicKey, "tsa-public-key", "File containing a public key to expect from the TSA.")
	RetireWorkerCommand.Flags().Var(retireWorkerCmd.TSA.WorkerPrivateKey, "tsa-worker-private-key", "File containing the private key to use when authenticating to the TSA.")

	RetireWorkerCommand.MarkFlagRequired("name")
	RetireWorkerCommand.MarkFlagRequired("tsa-host")
	RetireWorkerCommand.MarkFlagRequired("tsa-public-key")
	RetireWorkerCommand.MarkFlagRequired("tsa-worker-private-key")
}

type RetireWorkerConfig struct {
	TSA worker.TSAConfig

	WorkerName string
	WorkerTeam string
}

func ExecuteRetireWorker(cmd *cobra.Command, args []string) error {
	logger := lager.NewLogger("retire-worker")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	client := retireWorkerCmd.TSA.Client(atc.Worker{
		Name: retireWorkerCmd.WorkerName,
		Team: retireWorkerCmd.WorkerTeam,
	})

	return client.Retire(lagerctx.NewContext(context.Background(), logger))
}
