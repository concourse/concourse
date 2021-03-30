package tsacmd

import (
	"github.com/concourse/concourse/flag"
	"github.com/spf13/cobra"
)

// XXX !! IMPORTANT !!
// These flags exist purely for backwards compatibility. Any new fields added
// to the TSAConfig will NOT need to be added here because we do not want to
// support new fields as flags.

func InitializeTSAFlagsDEPRECATED(c *cobra.Command, flags *TSAConfig) {
	c.Flags().StringVar(&flags.Logger.LogLevel, "tsa-log-level", CmdDefaults.Logger.LogLevel, "Minimum level of logs to see.")

	c.Flags().IPVar(&flags.BindIP, "tsa-bind-ip", CmdDefaults.BindIP, "IP address on which to listen for SSH.")
	c.Flags().Uint16Var(&flags.BindPort, "tsa-bind-port", CmdDefaults.BindPort, "Port on which to listen for SSH.")
	c.Flags().StringVar(&flags.PeerAddress, "tsa-peer-address", CmdDefaults.PeerAddress, "Network address of this web node, reachable by other web nodes. Used for forwarded worker addresses.")

	c.Flags().IPVar(&flags.Debug.BindIP, "tsa-debug-bind-ip", CmdDefaults.Debug.BindIP, "IP address on which to listen for the pprof debugger endpoints.")
	c.Flags().Uint16Var(&flags.Debug.BindPort, "tsa-debug-bind-port", CmdDefaults.Debug.BindPort, "Port on which to listen for the pprof debugger endpoints.")

	var hostKey flag.PrivateKey
	c.Flags().Var(&hostKey, "tsa-host-key", "Path to private key to use for the SSH server.")
	if hostKey.PrivateKey != nil {
		flags.HostKey = &hostKey
	}

	c.Flags().Var(&flags.AuthorizedKeys, "tsa-authorized-keys", "Path to file containing keys to authorize, in SSH authorized_keys format (one public key per line).")
	c.Flags().Var(&flags.TeamAuthorizedKeys, "tsa-team-authorized-keys", "Path to file containing keys to authorize, in SSH authorized_keys format (one public key per line).")
	c.Flags().Var(&flags.TeamAuthorizedKeysFile, "tsa-team-authorized-keys-file", "Path to file containing a YAML array of teams and their authorized SSH keys, e.g. [{team:foo,ssh_keys:[key1,key2]}].")

	c.Flags().Var(&flags.ATCURLs, "tsa-atc-url", "ATC API endpoints to which workers will be registered.")

	c.Flags().StringVar(&flags.ClientID, "tsa-client-id", CmdDefaults.ClientID, "Client used to fetch a token from the auth server. NOTE: if you change this value you will also need to change the --system-claim-value flag so the atc knows to allow requests from this client.")
	c.Flags().StringVar(&flags.ClientSecret, "tsa-client-secret", "", "Client used to fetch a token from the auth server")
	c.Flags().Var(&flags.TokenURL, "tsa-token-url", "Token endpoint of the auth server")
	c.Flags().StringArrayVar(&flags.Scopes, "tsa-scope", nil, "Scopes to request from the auth server")

	c.Flags().DurationVar(&flags.HeartbeatInterval, "tsa-heartbeat-interval", CmdDefaults.HeartbeatInterval, "interval on which to heartbeat workers to the ATC")
	c.Flags().DurationVar(&flags.GardenRequestTimeout, "garden-request-timeout", CmdDefaults.GardenRequestTimeout, "How long to wait for requests to Garden to complete. 0 means no timeout.")

	c.Flags().StringVar(&flags.ClusterName, "tsa-cluster-name", "", "A name for this Concourse cluster, to be displayed on the dashboard page.")
	c.Flags().BoolVar(&flags.LogClusterName, "tsa-log-cluster-name", false, "Log cluster name.")
}
