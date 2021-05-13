package workercmd

import (
	"github.com/spf13/cobra"
)

// Any new fields DO NOT need to be added as a flag. This is purely for
// backwards compatibility only!
func InitializeRuntimeFlagsDEPRECATED(c *cobra.Command, flags *WorkerCommand, prefix string) {
	c.Flags().StringVar(&flags.Certs.Dir, prefix+"certs-dir", "", "Directory to use when creating the resource certificates volume.")

	c.Flags().StringVar(&flags.Runtime, prefix+"runtime", RuntimeDefaults.Runtime, "Runtime to use with the worker. Please note that Houdini is insecure and doesn't run 'tasks' in containers.")

	// Garden configuration
	c.Flags().StringVar(&flags.Guardian.Bin, prefix+"garden-bin", "", "Path to a garden server executable (non-absolute names get resolved from $PATH).")
	c.Flags().BoolVar(&flags.Guardian.DNS.Enable, prefix+"garden-dns-proxy-enable", false, "Enable proxy DNS server.")
	c.Flags().DurationVar(&flags.Guardian.RequestTimeout, prefix+"garden-request-timeout", GuardianDefaults.RequestTimeout, "How long to wait for requests to the Garden server to complete. 0 means no timeout.")
	c.Flags().Var(&flags.Guardian.Config, prefix+"garden-config", "Path to a config file to use for the Garden backend. e.g. 'foo-bar=a,b' for '--foo-bar a --foo-bar b'.")
	c.Flags().StringVar(&flags.Guardian.BinaryFlags.Server.Network.Pool, prefix+"garden-network-pool", "", "Network range to use for dynamically allocated container subnets. (default:10.80.0.0/16)")
	c.Flags().StringVar(&flags.Guardian.BinaryFlags.Server.Limits.MaxContainers, prefix+"garden-max-containers", "", "Maximum container capacity. 0 means no limit. (default:250)")

	// Containerd configuration
	c.Flags().Var(&flags.Containerd.Config, prefix+"containerd-config", "Path to a config file to use for the Containerd daemon.")
	c.Flags().StringVar(&flags.Containerd.Bin, prefix+"containerd-bin", "", "Path to a containerd executable (non-absolute names get resolved from $PATH).")
	c.Flags().StringVar(&flags.Containerd.InitBin, prefix+"containerd-init-bin", ContainerdDefaults.InitBin, "Path to an init executable (non-absolute names get resolved from $PATH).")
	c.Flags().StringVar(&flags.Containerd.CNIPluginsDir, prefix+"containerd-cni-plugins-dir", ContainerdDefaults.CNIPluginsDir, "Path to CNI network plugins.")
	c.Flags().DurationVar(&flags.Containerd.RequestTimeout, prefix+"containerd-request-timeout", ContainerdDefaults.RequestTimeout, "How long to wait for requests to Containerd to complete. 0 means no timeout.")
	c.Flags().IPVar(&flags.Containerd.Network.ExternalIP, prefix+"containerd-external-ip", nil, "IP address to use to reach container's mapped ports. Autodetected if not specified.")
	c.Flags().BoolVar(&flags.Containerd.Network.DNS.Enable, prefix+"containerd-dns-proxy-enable", false, "Enable proxy DNS server.")
	c.Flags().StringSliceVar(&flags.Containerd.Network.DNSServers, prefix+"containerd-dns-server", nil, "DNS server IP address to use instead of automatically determined servers. Can be specified multiple times.")
	c.Flags().StringSliceVar(&flags.Containerd.Network.RestrictedNetworks, prefix+"containerd-restricted-network", nil, "Network ranges to which traffic from containers will be restricted. Can be specified multiple times.")
	c.Flags().StringVar(&flags.Containerd.Network.Pool, prefix+"containerd-network-pool", ContainerdDefaults.Network.Pool, "Network range to use for dynamically allocated container subnets.")
	c.Flags().IntVar(&flags.Containerd.Network.MTU, prefix+"containerd-mtu", 0, "MTU size for container network interfaces. Defaults to the MTU of the interface used for outbound access by the host.")
	c.Flags().IntVar(&flags.Containerd.MaxContainers, prefix+"containerd-max-containers", ContainerdDefaults.MaxContainers, "Max container capacity. 0 means no limit.")
}
