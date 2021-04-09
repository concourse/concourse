package workercmd

import (
	"github.com/concourse/baggageclaim/baggageclaimcmd"
	"github.com/spf13/cobra"
)

func InitializeWorkerFlagsDEPRECATED(c *cobra.Command, flags *WorkerCommand) {
	InitializeWorkerFlags(c, flags)
	InitializeTSAConfigFlags(c, flags)

	c.Flags().Var(&flags.WorkDir, "work-dir", "Directory in which to place container data.")

	c.Flags().IPVar(&flags.BindIP, "bind-ip", CmdDefaults.BindIP, "IP address on which to listen for the Garden server.")
	c.Flags().Uint16Var(&flags.BindPort, "bind-port", CmdDefaults.BindPort, "Port on which to listen for the Garden server.")

	InitializeDebugConfigFlags(c, flags)
	InitializeHealthcheckConfigFlags(c, flags)

	c.Flags().DurationVar(&flags.SweepInterval, "sweep-interval", CmdDefaults.SweepInterval, "Interval on which containers and volumes will be garbage collected from the worker.")
	c.Flags().Uint16Var(&flags.VolumeSweeperMaxInFlight, "volume-sweeper-max-in-flight", CmdDefaults.VolumeSweeperMaxInFlight, "Maximum number of volumes which can be swept in parallel.")
	c.Flags().Uint16Var(&flags.ContainerSweeperMaxInFlight, "container-sweeper-max-in-flight", CmdDefaults.ContainerSweeperMaxInFlight, "Maximum number of containers which can be swept in parallel.")

	c.Flags().DurationVar(&flags.RebalanceInterval, "rebalance-interval", CmdDefaults.RebalanceInterval, "Duration after which the registration should be swapped to another random SSH gateway.")
	c.Flags().DurationVar(&flags.ConnectionDrainTimeout, "connection-drain-timeout", CmdDefaults.ConnectionDrainTimeout, "Duration after which a worker should give up draining forwarded connections on shutdown.")

	InitializeGuardianRuntimeFlags(c, flags)

	c.Flags().Var(&flags.ExternalGardenURL, "external-garden-url", "API endpoint of an externally managed Garden server to use instead of running the embedded Garden server.")

	baggageclaimcmd.InitializeBaggageclaimFlags(c, &flags.Baggageclaim)

	c.Flags().Var(&flags.ResourceTypes, "resource-types", "Path to directory containing resource types the worker should advertise.")

	c.Flags().StringVar(&flags.Logger.LogLevel, "log-level", CmdDefaults.Logger.LogLevel, "Minimum level of logs to see.")
}

func InitializeWorkerFlags(c *cobra.Command, flags *WorkerCommand) {
	c.Flags().StringVar(&flags.Worker.Name, "name", "", "The name to set for the worker during registration. If not specified, the hostname will be used.")
	c.Flags().StringSliceVar(&flags.Worker.Tags, "tag", nil, "A tag to set during registration. Can be specified multiple times.")
	c.Flags().StringVar(&flags.Worker.TeamName, "team", "", "The name of the team that this worker will be assigned to.")
	c.Flags().StringVar(&flags.Worker.HTTPProxy, "http-proxy", "", "HTTP proxy endpoint to use for containers.")
	c.Flags().StringVar(&flags.Worker.HTTPSProxy, "https-proxy", "", "HTTPS proxy endpoint to use for containers.")
	c.Flags().StringVar(&flags.Worker.NoProxy, "no-proxy", "", "Blacklist of addresses to skip the proxy when reaching.")
	c.Flags().BoolVar(&flags.Worker.Ephemeral, "ephemeral", false, "If set, the worker will be immediately removed upon stalling.")
	c.Flags().StringVar(&flags.Worker.Version, "version", "", "Version of the worker. This is normally baked in to the binary, so this flag is hidden.")
	c.Flags().MarkHidden("version")
}

func InitializeTSAConfigFlags(c *cobra.Command, flags *WorkerCommand) {
	c.Flags().StringSliceVar(&flags.TSA.Hosts, "tsa-host", CmdDefaults.TSA.Hosts, "TSA host to forward the worker through. Can be specified multiple times.")
	c.Flags().Var(&flags.TSA.PublicKey, "tsa-public-key", "File containing a public key to expect from the TSA.")
	c.Flags().Var(flags.TSA.WorkerPrivateKey, "tsa-worker-private-key", "File containing the private key to use when authenticating to the TSA.")
}

func InitializeDebugConfigFlags(c *cobra.Command, flags *WorkerCommand) {
	c.Flags().IPVar(&flags.Debug.BindIP, "debug-bind-ip", CmdDefaults.Debug.BindIP, "IP address on which to listen for the pprof debugger endpoints.")
	c.Flags().Uint16Var(&flags.Debug.BindPort, "debug-bind-port", CmdDefaults.Debug.BindPort, "Port on which to listen for the pprof debugger endpoints.")
}

func InitializeHealthcheckConfigFlags(c *cobra.Command, flags *WorkerCommand) {
	c.Flags().IPVar(&flags.Healthcheck.BindIP, "healthcheck-bind-ip", CmdDefaults.Healthcheck.BindIP, "IP address on which to listen for health checking requests.")
	c.Flags().Uint16Var(&flags.Healthcheck.BindPort, "healthcheck-bind-port", CmdDefaults.Healthcheck.BindPort, "Port on which to listen for health checking requests.")
	c.Flags().DurationVar(&flags.Healthcheck.Timeout, "healthcheck-timeout", CmdDefaults.Healthcheck.Timeout, "HTTP timeout for the full duration of health checking.")
}

func InitializeGuardianRuntimeFlags(c *cobra.Command, flags *WorkerCommand) {
	c.Flags().DurationVar(&flags.Guardian.RequestTimeout, "garden-request-timeout", CmdDefaults.Guardian.RequestTimeout, "How long to wait for requests to the Garden server to complete. 0 means no timeout.")
}
