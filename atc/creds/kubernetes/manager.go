package kubernetes

import (
	"encoding/json"
	"errors"

	"code.cloudfoundry.org/lager/v3"

	"github.com/concourse/concourse/v8/atc/creds"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesManager struct {
	InClusterConfig       bool   `long:"in-cluster" description:"Enables the in-cluster client."`
	ConfigPath            string `long:"config-path" description:"Path to Kubernetes config when running ATC outside Kubernetes."`
	NamespacePrefix       string `long:"namespace-prefix" default:"concourse-" description:"Prefix to use for Kubernetes namespaces under which secrets will be looked up."`
	NamespaceSharedSuffix string `long:"shared-namespace-suffix" description:"Appended to the namespace-prefix, which combined should match an existing Kubernetes namespace used to lookup shared secrets in."`
}

func (manager *KubernetesManager) MarshalJSON() ([]byte, error) {
	// XXX: Get Health
	return json.Marshal(&map[string]any{
		"in_cluster_config":       manager.InClusterConfig,
		"config_path":             manager.ConfigPath,
		"namespace_config":        manager.NamespacePrefix,
		"namespace_shared_config": manager.NamespaceSharedSuffix,
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

	return NewKubernetesFactory(logger, clientset, manager.NamespacePrefix, manager.NamespaceSharedSuffix), nil
}

func (manager KubernetesManager) Close(logger lager.Logger) {
	// TODO - to implement
}
