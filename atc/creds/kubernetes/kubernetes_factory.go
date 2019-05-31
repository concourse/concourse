package kubernetes

import (
	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/v5/atc/creds"
	"k8s.io/client-go/kubernetes"
)

type kubernetesFactory struct {
	clientset       *kubernetes.Clientset
	logger          lager.Logger
	namespacePrefix string
}

func NewKubernetesFactory(logger lager.Logger, clientset *kubernetes.Clientset, namespacePrefix string) *kubernetesFactory {
	factory := &kubernetesFactory{
		clientset:       clientset,
		logger:          logger,
		namespacePrefix: namespacePrefix,
	}

	return factory
}

func (factory *kubernetesFactory) NewSecrets() creds.Secrets {
	return &Kubernetes{
		Clientset:       factory.clientset,
		logger:          factory.logger,
		namespacePrefix: factory.namespacePrefix,
	}
}
