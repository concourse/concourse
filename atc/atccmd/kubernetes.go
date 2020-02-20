package atccmd

import (
	"fmt"

	"github.com/mitchellh/go-homedir"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesConfig struct {
	InCluster  bool   `long:"in-cluster"`
	Kubeconfig string `long:"kubeconfig" default:"~/.kube/config"`

	ClusterUrl string
	ClusterCA  string
	Token      string

	// TODO host, ca certs, token, etc
	//
}

func (k KubernetesConfig) Config() (cfg *rest.Config, err error) {
	switch {
	case k.InCluster:
		cfg, err = rest.InClusterConfig()
		if err != nil {
			err = fmt.Errorf("incluster cfg: %w", err)
			return
		}
	case k.Kubeconfig != "":
		var fpath string

		fpath, err = homedir.Expand(k.Kubeconfig)
		if err != nil {
			err = fmt.Errorf("path expansion: %w", err)
			return
		}

		cfg, err = clientcmd.BuildConfigFromFlags("", fpath)
		if err != nil {
			return
		}
	default:
		err = fmt.Errorf("kubeconfig or in-cluster must be specified")
		return
	}

	return
}
