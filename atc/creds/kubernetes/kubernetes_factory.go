package kubernetes

import (
	"code.cloudfoundry.org/lager"
	"k8s.io/client-go/kubernetes"

	"github.com/concourse/concourse/atc/creds"
)

type kubernetesFactory struct {
	logger lager.Logger

	client          kubernetes.Interface
	namespacePrefix string
}

func NewKubernetesFactory(logger lager.Logger, client kubernetes.Interface, namespacePrefix string) *kubernetesFactory {
	factory := &kubernetesFactory{
		logger:          logger,
		client:          client,
		namespacePrefix: namespacePrefix,
	}

	return factory
}

func (factory *kubernetesFactory) NewSecrets() creds.Secrets {
	return &Secrets{
		logger:          factory.logger,
		client:          factory.client,
		namespacePrefix: factory.namespacePrefix,
	}
}
