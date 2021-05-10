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

var landWorkerCmd LandWorkerConfig

var LandWorkerCommand = &cobra.Command{
	Use:   "land-worker",
	Short: "Safely drain a worker's assignments for temporary downtime",
	Long:  `TODO`,
	RunE:  ExecuteLandWorker,
}

func init() {
	LandWorkerCommand.Flags().StringVar(&landWorkerCmd.WorkerName, "name", "", "The name of the worker you wish to land.")
	LandWorkerCommand.Flags().StringSliceVar(&landWorkerCmd.TSA.Hosts, "tsa-host", []string{"127.0.0.1:2222"}, "TSA host to forward the worker through. Can be specified multiple times.")
	LandWorkerCommand.Flags().Var(&landWorkerCmd.TSA.PublicKey, "tsa-public-key", "File containing a public key to expect from the TSA.")
	LandWorkerCommand.Flags().Var(&landWorkerCmd.TSA.WorkerPrivateKey, "tsa-worker-private-key", "File containing the private key to use when authenticating to the TSA.")

	LandWorkerCommand.MarkFlagRequired("name")
	LandWorkerCommand.MarkFlagRequired("tsa-host")
	LandWorkerCommand.MarkFlagRequired("tsa-public-key")
	LandWorkerCommand.MarkFlagRequired("tsa-worker-private-key")
}

type LandWorkerConfig struct {
	WorkerName string

	TSA worker.TSAConfig
}

func ExecuteLandWorker(cmd *cobra.Command, args []string) error {
	logger := lager.NewLogger("land-worker")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	client := landWorkerCmd.TSA.Client(atc.Worker{
		Name: landWorkerCmd.WorkerName,
	})

	return client.Land(lagerctx.NewContext(context.Background(), logger))
}
