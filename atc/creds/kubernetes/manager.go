package kubernetes

import (
	"encoding/json"
	"errors"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/creds"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const managerName = "kubernetes"

type KubernetesManager struct {
	Enabled         bool   `yaml:"enabled,omitempty"`
	InClusterConfig bool   `yaml:"in_cluster,omitempty"`
	ConfigPath      string `yaml:"config_path,omitempty"`
	NamespacePrefix string `yaml:"namespace_prefix,omitempty"`
}

func (manager *KubernetesManager) MarshalJSON() ([]byte, error) {
	// XXX: Get Health
	return json.Marshal(&map[string]interface{}{
		"in_cluster_config": manager.InClusterConfig,
		"config_path":       manager.ConfigPath,
		"namespace_config":  manager.NamespacePrefix,
	})
}

func (manager *KubernetesManager) Name() string {
	return managerName
}

func (manager *KubernetesManager) Config() interface{} {
	return manager
}

func (manager KubernetesManager) Init(log lager.Logger) error {
	return nil
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
	if !manager.InClusterConfig && manager.ConfigPath == "" {
		return errors.New("Either in_cluster or config_path needs to be configured")
	}

	if manager.InClusterConfig && manager.ConfigPath != "" {
		return errors.New("either in-cluster or config-path can be used, not both")
	}
	_, err := manager.buildConfig()
	return err
}

func (manager KubernetesManager) NewSecretsFactory(logger lager.Logger) (creds.SecretsFactory, error) {
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

func (manager KubernetesManager) Close(logger lager.Logger) {
	// TODO - to implement
}
