package concourse

type WebConfig struct {
	PeerAddress string `long:"peer-address" default:"127.0.0.1" description:"Network address of this web node, reachable by other web nodes. Used for forwarded worker addresses."`
	ClusterName    string `long:"cluster-name" description:"A name for this Concourse cluster, to be displayed on the dashboard page."`
	LogClusterName bool   `long:"log-cluster-name" description:"Log cluster name."`
}
