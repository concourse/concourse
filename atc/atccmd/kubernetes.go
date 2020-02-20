package atccmd

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/cert"
)

type KubernetesTargetConfig struct {
	RestConfig *rest.Config
	Namespace  string
}

// 	tokenFile  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
// 	rootCAFile = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
func fromServiceAccount(mountpoint string) (*KubernetesTargetConfig, error) {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if len(host) == 0 || len(port) == 0 {
		return nil, fmt.Errorf("not in cluster")
	}

	namespacePath := filepath.Join(mountpoint, "namespace")
	tokenPath := filepath.Join(mountpoint, "token")
	rootcaPath := filepath.Join(mountpoint, "ca.crt")

	namespaceBytes, err := ioutil.ReadFile(namespacePath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", namespacePath, err)
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

	return &KubernetesTargetConfig{
		RestConfig: &rest.Config{
			Host:            "https://" + net.JoinHostPort(host, port),
			TLSClientConfig: tlsClientConfig,
			BearerToken:     string(token),
			BearerTokenFile: tokenPath,
		},
		Namespace: string(namespaceBytes),
	}, nil
}

func fromKubeConfig(fpath string) (*KubernetesTargetConfig, error) {
	fpath, err := homedir.Expand(fpath)
	if err != nil {
		return nil, fmt.Errorf("path expansion: %w", err)
	}

	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{Precedence: []string{fpath}},
		&clientcmd.ConfigOverrides{},
	)

	config, err := cc.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("client config: %w", err)
	}

	namespace, found, err := cc.Namespace()
	if err != nil {
		return nil, fmt.Errorf("namespace: %w", err)
	}

	if !found {
		return nil, fmt.Errorf("kubeconfig %s without namespace specified", fpath)
	}

	return &KubernetesTargetConfig{
		RestConfig: config,
		Namespace:  namespace,
	}, nil
}

func (k KubernetesConfig) Config() (cfg *KubernetesTargetConfig, err error) {
	switch {
	case k.ServiceAccount != "":
		cfg, err = fromServiceAccount(k.ServiceAccount)
		if err != nil {
			err = fmt.Errorf("from service account: %w", err)
			return
		}
	case k.Kubeconfig != "":
		cfg, err = fromKubeConfig(k.Kubeconfig)
		if err != nil {
			err = fmt.Errorf("from kubeconfig: %w", err)
			return
		}
	default:
		err = fmt.Errorf("kubeconfig or service-account must be specified")
		return
	}

	return
}
