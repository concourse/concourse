package kubernetes

import (
	"github.com/concourse/atc/creds"
	flags "github.com/jessevdk/go-flags"
)

type kubernetesManagerFactory struct{}

func init() {
	creds.Register("kubernetes", NewKubernetesManagerFactory())
}

func NewKubernetesManagerFactory() creds.ManagerFactory {
	return &kubernetesManagerFactory{}
}

func (factory *kubernetesManagerFactory) AddConfig(group *flags.Group) creds.Manager {
	manager := &KubernetesManager{}

	subGroup, err := group.AddGroup("Kubernetes Credential Management", "", manager)
	if err != nil {
		panic(err)
	}

	subGroup.Namespace = "kubernetes"

	return manager
}
