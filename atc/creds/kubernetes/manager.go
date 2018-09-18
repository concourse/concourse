package kubernetes

import (
	"encoding/json"
	"errors"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/creds"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesManager struct {
	InClusterConfig bool   `long:"in-cluster" description:"Enables the in-cluster client."`
	ConfigPath      string `long:"config-path" description:"Path to Kubernetes config when running ATC outside Kubernetes."`
	NamespacePrefix string `long:"namespace-prefix" default:"concourse-" description:"Prefix to use for Kubernetes namespaces under which secrets will be looked up."`
}

func (manager *KubernetesManager) MarshalJSON() ([]byte, error) {
	// XXX: Get Health
	return json.Marshal(&map[string]interface{}{
		"in_cluster_config": manager.InClusterConfig,
		"config_path":       manager.ConfigPath,
		"namespace_config":  manager.NamespacePrefix,
	})
}

func (manager KubernetesManager) Init(log lager.Logger) error {
	return nil
}

func (manager KubernetesManager) IsConfigured() bool {
	return manager.InClusterConfig || manager.ConfigPath != ""
}

func (manager KubernetesManager) buildConfig() (*rest.Config, error) {
	if manager.InClusterConfig {
		return rest.InClusterConfig()
	}

	return clientcmd.BuildConfigFromFlags("", manager.ConfigPath)
}

func (manager KubernetesManager) Health() (*creds.HealthResponse, error) {
	return nil, nil
}

func (manager KubernetesManager) Validate() error {
	if manager.InClusterConfig && manager.ConfigPath != "" {
		return errors.New("Either in-cluster or config-path can be used, not both.")
	}
	_, err := manager.buildConfig()
	return err
}

func (manager KubernetesManager) NewVariablesFactory(logger lager.Logger) (creds.VariablesFactory, error) {
	config, err := manager.buildConfig()
	if err != nil {
		return nil, err
	}

	config.QPS = 100
	config.Burst = 100

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return NewKubernetesFactory(logger, clientset, manager.NamespacePrefix), nil
}
