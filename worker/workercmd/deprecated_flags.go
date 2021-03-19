package workercmd

import (
	"net"
	"time"

	"github.com/spf13/cobra"
)

func InitializeWorkerFlagsDEPRECATED(c *cobra.Command, flags *WorkerCommand) {
	InitializeWorkerFlags(c, flags)
	InitializeTSAConfigFlags(c, flags)

	c.Flags().Var(&flags.WorkDir, "work-dir", "Directory in which to place container data.")

	c.Flags().IPVar(&flags.BindIP, "bind-ip", net.IPv4(127, 0, 0, 1), "IP address on which to listen for the Garden server.")
	c.Flags().Uint16Var(&flags.BindPort, "bind-port", 7777, "Port on which to listen for the Garden server.")

	InitializeDebugConfigFlags(c, flags)

	c.Flags().DurationVar(&flags.SweepInterval, "sweep-interval", 30*time.Second, "Interval on which containers and volumes will be garbage collected from the worker.")
	c.Flags().Uint16Var(&flags.VolumeSweeperMaxInFlight, "volume-sweeper-max-in-flight", 3, "Maximum number of volumes which can be swept in parallel.")
	c.Flags().Uint16Var(&flags.ContainerSweeperMaxInFlight, "container-sweeper-max-in-flight", 5, "Maximum number of containers which can be swept in parallel.")

	c.Flags().DurationVar(&flags.RebalanceInterval, "rebalance-interval", 4*time.Hour, "Duration after which the registration should be swapped to another random SSH gateway.")
	c.Flags().DurationVar(&flags.ConnectionDrainTimeout, "connection-drain-timeout", 1*time.Hour, "Duration after which a worker should give up draining forwarded connections on shutdown.")

	InitializeGuardianRuntimeFlags(c, flags)

	c.Flags().Var(&flags.ExternalGardenURL, "external-garden-url", "API endpoint of an externally managed Garden server to use instead of running the embedded Garden server.")

	InitializeBaggageclaimFlags(c, flags)

	c.Flags().Var(&flags.ResourceTypes, "resource-types", "Path to directory containing resource types the worker should advertise.")
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
	c.Flags().StringSliceVar(&flags.TSA.Hosts, "tsa-host", []string{"127.0.0.1:2222"}, "TSA host to forward the worker through. Can be specified multiple times.")
	c.Flags().Var(&flags.TSA.PublicKey, "tsa-public-key", "File containing a public key to expect from the TSA.")
	c.Flags().Var(flags.TSA.WorkerPrivateKey, "tsa-worker-private-key", "File containing the private key to use when authenticating to the TSA.")
}

func InitializeDebugConfigFlags(c *cobra.Command, flags *WorkerCommand) {
	c.Flags().IPVar(&flags.Debug.BindIP, "debug-bind-ip", net.IPv4(127, 0, 0, 1), "IP address on which to listen for the pprof debugger endpoints.")
	c.Flags().Uint16Var(&flags.Debug.BindPort, "debug-bind-port", 7776, "Port on which to listen for the pprof debugger endpoints.")
}

func InitializeHealthcheckConfigFlags(c *cobra.Command, flags *WorkerCommand) {
	c.Flags().IPVar(&flags.Healthcheck.BindIP, "healthcheck-bind-ip", net.IPv4(0, 0, 0, 0), "IP address on which to listen for health checking requests.")
	c.Flags().Uint16Var(&flags.Healthcheck.BindPort, "healthcheck-bind-port", 8888, "Port on which to listen for health checking requests.")
	c.Flags().DurationVar(&flags.Healthcheck.Timeout, "healthcheck-timeout", 5*time.Second, "HTTP timeout for the full duration of health checking.")
}

func InitializeGuardianRuntimeFlags(c *cobra.Command, flags *WorkerCommand) {
	c.Flags().DurationVar(&flags.Guardian.RequestTimeout, "garden-request-timeout", 5*time.Minute, "How long to wait for requests to the Garden server to complete. 0 means no timeout.")
}

func InitializeBaggageclaimFlags(c *cobra.Command, flags *WorkerCommand) {
	c.Flags().IPVar(&flags.Baggageclaim.BindIP, "baggageclaim-bind-ip", net.IPv4(127, 0, 0, 1), "IP address on which to listen for API traffic.")
	c.Flags().Uint16Var(&flags.Baggageclaim.BindPort, "baggageclaim-bind-port", 7788, "Port on which to listen for API traffic.")

	c.Flags().IPVar(&flags.Baggageclaim.Debug.BindIP, "baggageclaim-debug-bind-ip", net.IPv4(127, 0, 0, 1), "IP address on which to listen for the pprof debugger endpoints.")
	c.Flags().Uint16Var(&flags.Baggageclaim.Debug.BindPort, "baggageclaim-debug-bind-port", 7787, "Port on which to listen for the pprof debugger endpoints.")

	c.Flags().StringVar(&flags.Baggageclaim.P2p.InterfaceNamePattern, "baggageclaim-p2p-interface-name-pattern", "eth0", "Regular expression to match a network interface for p2p streaming")
	c.Flags().IntVar(&flags.Baggageclaim.P2p.InterfaceFamily, "baggageclaim-p2p-interface-family", 4, "4 for IPv4 and 6 for IPv6")

	c.Flags().Var(&flags.Baggageclaim.VolumesDir, "baggageclaim-volumes", "Directory in which to place volume data.")

	c.Flags().StringVar(&flags.Baggageclaim.Driver, "baggageclaim-driver", "detect", "Driver to use for managing volumes.")

	c.Flags().StringVar(&flags.Baggageclaim.BtrfsBin, "baggageclaim-btrfs-bin", "btrfs", "Path to btrfs binary")
	c.Flags().StringVar(&flags.Baggageclaim.MkfsBin, "baggageclaim-mkfs-bin", "mkfs.btrfs", "Path to mkfs.btrfs binary")

	c.Flags().StringVar(&flags.Baggageclaim.OverlaysDir, "baggageclaim-overlays-dir", "", "Path to directory in which to store overlay data")

	c.Flags().BoolVar(&flags.Baggageclaim.DisableUserNamespaces, "baggageclaim-disable-user-namespaces", false, "Disable remapping of user/group IDs in unprivileged volumes.")
}
