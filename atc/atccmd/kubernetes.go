package atccmd

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"github.com/mitchellh/go-homedir"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/cert"
)

type KubernetesConfig struct {
	ServiceAccount string `long:"service-account" description:"location of the service account mount"`
	Kubeconfig     string `long:"kubeconfig" default:"~/.kube/config" description:"kubeconfig file location"`
}

func (k KubernetesConfig) Config() (cfg *rest.Config, err error) {
	switch {
	case k.ServiceAccount != "":
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
		err = fmt.Errorf("kubeconfig or service-account must be specified")
		return
	}

	return
}

// const (
// 	tokenFile  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
// 	rootCAFile = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
// )
func InClusterConfig(tokenPath, rootcaPath string) (*rest.Config, error) {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if len(host) == 0 || len(port) == 0 {
		return nil, fmt.Errorf("not in cluster")
	}

	token, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", tokenPath, err)
	}

	_, err = cert.NewPool(rootcaPath)
	if err != nil {
		return nil, fmt.Errorf("root ca %s: %w", rootcaPath, err)
	}

	tlsClientConfig := rest.TLSClientConfig{
		CAFile: rootcaPath,
	}

	return &rest.Config{
		Host:            "https://" + net.JoinHostPort(host, port),
		TLSClientConfig: tlsClientConfig,
		BearerToken:     string(token),
		BearerTokenFile: tokenPath,
	}, nil
}
