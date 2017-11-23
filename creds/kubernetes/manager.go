package kubernetes

import (
	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/creds"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesManager struct {
	ConfigPath string `long:"config-path" description:"Path to kubernetes config. Leave empty for in-cluster ATC."`
	NamespacePrefix string `long:"namespace-prefix" default:"concourse-" description:"Prefix to use for Kubernetes namespaces under which secrets will be looked up."`
}

func (manager KubernetesManager) IsConfigured() bool {
	if manager.ConfigPath != "" {
		return true
	}
	_, err := rest.InClusterConfig()
	if err == nil {
		return true
	}
	return false
}

func (manager KubernetesManager) buildConfig() (*rest.Config, error) {
	if manager.ConfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", manager.ConfigPath)
	}
	return rest.InClusterConfig()
}

func (manager KubernetesManager) Validate() error {
	_, err := manager.buildConfig()
	return err
}

func (manager KubernetesManager) NewVariablesFactory(logger lager.Logger) (creds.VariablesFactory, error) {
	config, err := manager.buildConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return NewKubernetesFactory(logger, clientset, manager.NamespacePrefix), nil
}
