package kubernetes

import (
	"github.com/concourse/concourse/atc/creds"
)

type kubernetesManagerFactory struct{}

func init() {
	creds.Register("kubernetes", NewKubernetesManagerFactory())
}

func NewKubernetesManagerFactory() creds.ManagerFactory {
	return &kubernetesManagerFactory{}
}

func (factory *kubernetesManagerFactory) NewConfig() creds.ManagerConfig {
	return creds.ManagerConfig{
		Namespace:   "kubernetes",
		Description: "Kubernetes Credential Management",
		Manager:     &KubernetesManager{},
	}
}

func (factory *kubernetesManagerFactory) NewInstance(config any) (creds.Manager, error) {
	return &KubernetesManager{}, nil
}
