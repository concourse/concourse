package kubernetes

import (
	"github.com/concourse/concourse/atc/creds"
)

type kubernetesManagerFactory struct{}

func NewKubernetesManagerFactory() creds.ManagerFactory {
	return &kubernetesManagerFactory{}
}

func (factory *kubernetesManagerFactory) NewInstance(config interface{}) (creds.Manager, error) {
	return &KubernetesManager{}, nil
}
